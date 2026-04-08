package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type stdioClient struct {
	serverName string
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     *bufio.Reader
	stderrMu   sync.Mutex
	stderrText string
	nextID     int
	mu         sync.Mutex
}

const stdioRequestTimeout = 20 * time.Second

func newStdioClient(serverName string, cfg ServerConfig) (*stdioClient, error) {
	if strings.TrimSpace(cfg.Command) == "" {
		return nil, fmt.Errorf("stdio mcp server %s missing command", serverName)
	}
	cmd := exec.Command(cfg.Command, cfg.Args...)
	if cfg.CWD != "" {
		cmd.Dir = filepath.Clean(cfg.CWD)
	}
	cmd.Env = os.Environ()
	for k, v := range cfg.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		_ = stdin.Close()
		_ = stdoutPipe.Close()
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdoutPipe.Close()
		_ = stderrPipe.Close()
		return nil, err
	}

	client := &stdioClient{
		serverName: serverName,
		cmd:        cmd,
		stdin:      stdin,
		stdout:     bufio.NewReader(stdoutPipe),
		nextID:     0,
	}
	go func() {
		stderrBytes, _ := io.ReadAll(stderrPipe)
		client.stderrMu.Lock()
		client.stderrText = string(stderrBytes)
		client.stderrMu.Unlock()
	}()

	if err := client.initialize(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

func (c *stdioClient) ListTools() ([]ToolInfo, error) {
	result, err := c.call("tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Tools []struct {
			Name        string         `json:"name"`
			Description string         `json:"description"`
			InputSchema map[string]any `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		return nil, err
	}
	tools := make([]ToolInfo, 0, len(parsed.Tools))
	for _, tool := range parsed.Tools {
		tools = append(tools, ToolInfo{ServerName: c.serverName, Name: tool.Name, Description: tool.Description, InputSchema: tool.InputSchema})
	}
	return tools, nil
}

func (c *stdioClient) ListResources() ([]ResourceInfo, error) {
	result, err := c.call("resources/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Resources []struct {
			Name        string `json:"name"`
			URI         string `json:"uri"`
			Description string `json:"description"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		return nil, err
	}
	resources := make([]ResourceInfo, 0, len(parsed.Resources))
	for _, r := range parsed.Resources {
		name := r.Name
		if strings.TrimSpace(name) == "" {
			name = r.URI
		}
		resources = append(resources, ResourceInfo{ServerName: c.serverName, Name: name, URI: r.URI, Description: r.Description})
	}
	return resources, nil
}

func (c *stdioClient) CallTool(toolName string, arguments map[string]any) (string, error) {
	result, err := c.call("tools/call", map[string]any{"name": toolName, "arguments": arguments})
	if err != nil {
		return "", err
	}
	var parsed struct {
		Content           []map[string]any `json:"content"`
		StructuredContent any              `json:"structuredContent"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		return "", err
	}
	parts := make([]string, 0)
	for _, item := range parsed.Content {
		if typ, _ := item["type"].(string); typ == "text" {
			if text, _ := item["text"].(string); text != "" {
				parts = append(parts, text)
				continue
			}
		}
		b, _ := json.Marshal(item)
		parts = append(parts, string(b))
	}
	if len(parts) == 0 && parsed.StructuredContent != nil {
		parts = append(parts, fmt.Sprintf("%v", parsed.StructuredContent))
	}
	if len(parts) == 0 {
		parts = append(parts, "(no output)")
	}
	return strings.TrimSpace(strings.Join(parts, "\n")), nil
}

func (c *stdioClient) ReadResource(uri string) (string, error) {
	result, err := c.call("resources/read", map[string]any{"uri": uri})
	if err != nil {
		return "", err
	}
	var parsed struct {
		Contents []map[string]any `json:"contents"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		return "", err
	}
	parts := make([]string, 0, len(parsed.Contents))
	for _, item := range parsed.Contents {
		if text, _ := item["text"].(string); text != "" {
			parts = append(parts, text)
			continue
		}
		if blob, _ := item["blob"].(string); blob != "" {
			parts = append(parts, blob)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n")), nil
}

func (c *stdioClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
		_, _ = c.cmd.Process.Wait()
	}
	return nil
}

func (c *stdioClient) initialize() error {
	_, err := c.call("initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "goharness",
			"version": "0.1.0",
		},
	})
	if err != nil {
		return err
	}
	return c.notify("initialized", map[string]any{})
}

func (c *stdioClient) notify(method string, params map[string]any) error {
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
	if err != nil {
		return err
	}
	return c.writeFrame(body)
}

func (c *stdioClient) call(method string, params map[string]any) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nextID++
	id := c.nextID
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	if err != nil {
		return nil, err
	}
	if err := c.writeFrame(body); err != nil {
		return nil, err
	}
	for {
		frame, err := c.readFrameWithTimeout(stdioRequestTimeout)
		if err != nil {
			return nil, c.wrapError(err)
		}
		var resp struct {
			ID     *int            `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(frame, &resp); err != nil {
			continue
		}
		if resp.ID == nil || *resp.ID != id {
			continue
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("mcp error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	}
}

func (c *stdioClient) wrapError(err error) error {
	c.stderrMu.Lock()
	stderr := strings.TrimSpace(c.stderrText)
	c.stderrMu.Unlock()
	if stderr == "" {
		return err
	}
	if len(stderr) > 500 {
		stderr = stderr[:500] + "..."
	}
	return fmt.Errorf("%w (stderr: %s)", err, stderr)
}

func (c *stdioClient) writeFrame(body []byte) error {
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	if _, err := io.WriteString(c.stdin, header); err != nil {
		return err
	}
	_, err := c.stdin.Write(body)
	return err
}

func (c *stdioClient) readFrame() ([]byte, error) {
	contentLength := -1
	for {
		line, err := c.stdout.ReadString('\n')
		if err != nil {
			return nil, err
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			break
		}
		if strings.HasPrefix(strings.ToLower(trimmed), "content-length:") {
			value := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(trimmed), "content-length:"))
			n, err := strconv.Atoi(value)
			if err != nil {
				return nil, err
			}
			contentLength = n
		}
	}
	if contentLength < 0 {
		return nil, fmt.Errorf("missing content-length header in mcp response")
	}
	buf := make([]byte, contentLength)
	if _, err := io.ReadFull(c.stdout, buf); err != nil {
		return nil, err
	}
	return bytes.TrimSpace(buf), nil
}

func (c *stdioClient) readFrameWithTimeout(timeout time.Duration) ([]byte, error) {
	type readResult struct {
		body []byte
		err  error
	}
	ch := make(chan readResult, 1)
	go func() {
		body, err := c.readFrame()
		ch <- readResult{body: body, err: err}
	}()
	select {
	case res := <-ch:
		return res.body, res.err
	case <-time.After(timeout):
		return nil, fmt.Errorf("timed out waiting for MCP response after %s", timeout)
	}
}
