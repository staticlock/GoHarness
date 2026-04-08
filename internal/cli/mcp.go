package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/user/goharness/internal/config"
)

func newMCPCommand() *cobra.Command {
	mcpCmd := &cobra.Command{Use: "mcp", Short: "Manage MCP servers"}

	mcpCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List configured MCP servers",
		RunE: func(_ *cobra.Command, _ []string) error {
			settings, err := config.LoadSettings()
			if err != nil {
				return err
			}
			if len(settings.MCPServers) == 0 {
				fmt.Println("No MCP servers configured.")
				return nil
			}
			for name, cfg := range settings.MCPServers {
				fmt.Printf("  %s: %v\n", name, cfg)
			}
			return nil
		},
	})

	var name string
	var configJSON string
	addCmd := &cobra.Command{
		Use:   "add [name] [config_json]",
		Short: "Add an MCP server configuration",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) >= 1 {
				name = args[0]
			}
			if len(args) >= 2 {
				configJSON = args[1]
			}
			if name == "" || configJSON == "" {
				return fmt.Errorf("usage: mcp add <name> <config_json>")
			}
			settings, err := config.LoadSettings()
			if err != nil {
				return err
			}
			var cfg map[string]any
			if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
				return fmt.Errorf("invalid JSON: %w", err)
			}
			if settings.MCPServers == nil {
				settings.MCPServers = map[string]interface{}{}
			}
			settings.MCPServers[name] = cfg
			if err := config.SaveSettings(settings); err != nil {
				return err
			}
			fmt.Printf("Added MCP server: %s\n", name)
			return nil
		},
	}
	addCmd.Flags().StringVar(&name, "name", "", "Server name")
	addCmd.Flags().StringVar(&configJSON, "config-json", "", "Server config as JSON string")
	mcpCmd.AddCommand(addCmd)

	var removeName string
	removeCmd := &cobra.Command{
		Use:   "remove [name]",
		Short: "Remove an MCP server configuration",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) >= 1 {
				removeName = args[0]
			}
			if removeName == "" {
				return fmt.Errorf("usage: mcp remove <name>")
			}
			settings, err := config.LoadSettings()
			if err != nil {
				return err
			}
			if _, ok := settings.MCPServers[removeName]; !ok {
				return fmt.Errorf("MCP server not found: %s", removeName)
			}
			delete(settings.MCPServers, removeName)
			if err := config.SaveSettings(settings); err != nil {
				return err
			}
			fmt.Printf("Removed MCP server: %s\n", removeName)
			return nil
		},
	}
	removeCmd.Flags().StringVar(&removeName, "name", "", "Server name to remove")
	mcpCmd.AddCommand(removeCmd)

	return mcpCmd
}
