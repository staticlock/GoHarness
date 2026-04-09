package memory

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type MemoryHeader struct {
	Path        string
	Title       string
	Description string
	ModifiedAt  float64
}

func GetProjectMemoryDir(cwd string) (string, error) {
	resolved, err := filepath.Abs(cwd)
	if err != nil {
		return "", err
	}
	digest := hashFirst12(resolved)
	dir := filepath.Join("memory", filepath.Base(resolved)+"-"+digest)

	configDir, err := getDataDir()
	if err != nil {
		return "", err
	}
	memoryDir := filepath.Join(configDir, dir)
	if err := os.MkdirAll(memoryDir, 0755); err != nil {
		return "", err
	}
	return memoryDir, nil
}

func GetMemoryEntrypoint(cwd string) (string, error) {
	memoryDir, err := GetProjectMemoryDir(cwd)
	if err != nil {
		return "", err
	}
	return filepath.Join(memoryDir, "MEMORY.md"), nil
}

func ListMemoryFiles(cwd string) ([]string, error) {
	memoryDir, err := GetProjectMemoryDir(cwd)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(memoryDir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".md" {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)
	return files, nil
}

func AddMemoryEntry(cwd string, title string, content string) (string, error) {
	memoryDir, err := GetProjectMemoryDir(cwd)
	if err != nil {
		return "", err
	}

	slug := sanitizeSlug(title)
	if slug == "" {
		slug = "memory"
	}
	path := filepath.Join(memoryDir, slug+".md")

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}

	entrypoint, err := GetMemoryEntrypoint(cwd)
	if err != nil {
		return "", err
	}

	var existing string
	if data, err := os.ReadFile(entrypoint); err == nil {
		existing = string(data)
	} else if !os.IsNotExist(err) {
		return "", err
	} else {
		existing = "# Memory Index\n"
	}

	if !strings.Contains(existing, slug+".md") {
		existing = strings.TrimRight(existing, "\r\n") + "\n- [" + title + "](" + slug + ".md)\n"
		if err := os.WriteFile(entrypoint, []byte(existing), 0644); err != nil {
			return "", err
		}
	}

	return path, nil
}

func RemoveMemoryEntry(cwd string, name string) (bool, error) {
	memoryDir, err := GetProjectMemoryDir(cwd)
	if err != nil {
		return false, err
	}

	matches := []string{}
	pattern := name
	if !strings.HasSuffix(name, ".md") {
		pattern = name + ".md"
	}

	nameLower := strings.ToLower(name)
	patternLower := strings.ToLower(pattern)

	if entries, err := os.ReadDir(memoryDir); err == nil {
		for _, entry := range entries {
			entryLower := strings.ToLower(entry.Name())
			if entryLower == patternLower || entryLower == nameLower {
				matches = append(matches, entry.Name())
			}
		}
	}

	if len(matches) == 0 {
		return false, nil
	}

	path := filepath.Join(memoryDir, matches[0])
	if err := os.Remove(path); err != nil {
		return false, err
	}

	entrypoint, err := GetMemoryEntrypoint(cwd)
	if err != nil {
		return true, nil
	}

	if data, err := os.ReadFile(entrypoint); err == nil {
		lines := strings.Split(string(data), "\n")
		newLines := []string{}
		for _, line := range lines {
			if !strings.Contains(line, matches[0]) {
				newLines = append(newLines, line)
			}
		}
		os.WriteFile(entrypoint, []byte(strings.Join(newLines, "\n")), 0644)
	}

	return true, nil
}

func ScanMemoryFiles(cwd string, maxFiles int) []MemoryHeader {
	memoryDir, err := GetProjectMemoryDir(cwd)
	if err != nil {
		return nil
	}

	entries, err := os.ReadDir(memoryDir)
	if err != nil {
		return nil
	}

	var headers []MemoryHeader
	count := 0
	for _, entry := range entries {
		if count >= maxFiles {
			break
		}
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}

		path := filepath.Join(memoryDir, entry.Name())
		info, err := os.Stat(path)
		if err != nil {
			continue
		}

		title := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		desc := extractDescription(path)

		headers = append(headers, MemoryHeader{
			Path:        path,
			Title:       title,
			Description: desc,
			ModifiedAt:  float64(info.ModTime().Unix()),
		})
		count++
	}

	sort.Slice(headers, func(i, j int) bool {
		return headers[i].ModifiedAt > headers[j].ModifiedAt
	})

	return headers
}

func FindRelevantMemories(query string, cwd string, maxResults int) []MemoryHeader {
	tokens := extractTokens(query)
	if len(tokens) == 0 {
		return nil
	}

	headers := ScanMemoryFiles(cwd, 100)

	type scored struct {
		score  int
		header MemoryHeader
	}

	var scoredList []scored
	for _, header := range headers {
		haystack := strings.ToLower(header.Title + " " + header.Description)
		score := 0
		for _, token := range tokens {
			if strings.Contains(haystack, token) {
				score++
			}
		}
		if score > 0 {
			scoredList = append(scoredList, scored{score: score, header: header})
		}
	}

	sort.Slice(scoredList, func(i, j int) bool {
		if scoredList[i].score != scoredList[j].score {
			return scoredList[i].score > scoredList[j].score
		}
		return scoredList[i].header.ModifiedAt > scoredList[j].header.ModifiedAt
	})

	result := make([]MemoryHeader, 0, maxResults)
	for i := 0; i < len(scoredList) && i < maxResults; i++ {
		result = append(result, scoredList[i].header)
	}

	return result
}

func LoadMemoryPrompt(cwd string) (string, error) {
	memoryDir, err := GetProjectMemoryDir(cwd)
	if err != nil {
		return "", err
	}
	entrypoint, err := GetMemoryEntrypoint(cwd)
	if err != nil {
		return "", err
	}

	lines := []string{
		"# Memory",
		"- Persistent memory directory: " + memoryDir,
		"- Use this directory to store durable user or project context that should survive future sessions.",
		"- Prefer concise topic files plus an index entry in MEMORY.md.",
	}

	if _, err := os.Stat(entrypoint); err == nil {
		data, err := os.ReadFile(entrypoint)
		if err == nil {
			content := string(data)
			lines = append(lines, "", "## MEMORY.md", "```md", content, "```")
		}
	} else {
		lines = append(lines, "", "## MEMORY.md", "(not created yet)")
	}

	return strings.Join(lines, "\n"), nil
}

func hashFirst12(s string) string {
	h := 0
	for i, c := range s {
		h = h*31 + int(c)
		if i >= 100 {
			break
		}
	}
	result := ""
	for h > 0 {
		result = string(rune('a'+h%26)) + result
		h /= 26
	}
	if len(result) < 12 {
		padding := strings.Repeat("0", 12-len(result))
		result = padding + result
	}
	return result[:12]
}

func sanitizeSlug(title string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	result := re.ReplaceAllString(title, "_")
	result = strings.Trim(result, "_")
	if result == "" {
		return ""
	}
	return result
}

func extractDescription(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	content := string(data)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimLeft(line, "# ")
		line = strings.TrimSpace(line)
		if len(line) > 10 {
			if len(line) > 100 {
				line = line[:100] + "..."
			}
			return line
		}
	}
	return ""
}

func getDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".openharness", "data"), nil
}

func extractTokens(query string) []string {
	re := regexp.MustCompile(`[A-Za-z0-9_]+`)
	matches := re.FindAllString(query, -1)
	var tokens []string
	for _, token := range matches {
		if len(token) >= 3 {
			tokens = append(tokens, strings.ToLower(token))
		}
	}
	return tokens
}
