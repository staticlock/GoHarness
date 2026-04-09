package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/staticlock/GoHarness/internal/config"
)

func newPluginCommand() *cobra.Command {
	pluginCmd := &cobra.Command{Use: "plugin", Short: "Manage plugins"}

	pluginCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List installed plugins",
		RunE: func(_ *cobra.Command, _ []string) error {
			settings, err := config.LoadSettings()
			if err != nil {
				return err
			}
			if len(settings.EnabledPlugins) == 0 {
				fmt.Println("No plugins installed.")
				return nil
			}
			for name, enabled := range settings.EnabledPlugins {
				status := "disabled"
				if enabled {
					status = "enabled"
				}
				fmt.Printf("  %s [%s]\n", name, status)
			}
			return nil
		},
	})

	var source string
	installCmd := &cobra.Command{
		Use:   "install [source]",
		Short: "Install a plugin from a source path",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) >= 1 {
				source = args[0]
			}
			if source == "" {
				return fmt.Errorf("usage: plugin install <source>")
			}
			settings, err := config.LoadSettings()
			if err != nil {
				return err
			}
			if settings.EnabledPlugins == nil {
				settings.EnabledPlugins = map[string]bool{}
			}
			name := strings.TrimSuffix(filepath.Base(source), filepath.Ext(source))
			if name == "" || name == "." {
				return fmt.Errorf("invalid plugin source: %s", source)
			}
			settings.EnabledPlugins[name] = true
			if err := config.SaveSettings(settings); err != nil {
				return err
			}
			fmt.Printf("Installed plugin: %s\n", name)
			return nil
		},
	}
	installCmd.Flags().StringVar(&source, "source", "", "Plugin source (path or URL)")
	pluginCmd.AddCommand(installCmd)

	var name string
	uninstallCmd := &cobra.Command{
		Use:   "uninstall [name]",
		Short: "Uninstall a plugin",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) >= 1 {
				name = args[0]
			}
			if name == "" {
				return fmt.Errorf("usage: plugin uninstall <name>")
			}
			settings, err := config.LoadSettings()
			if err != nil {
				return err
			}
			delete(settings.EnabledPlugins, name)
			if err := config.SaveSettings(settings); err != nil {
				return err
			}
			fmt.Printf("Uninstalled plugin: %s\n", name)
			return nil
		},
	}
	uninstallCmd.Flags().StringVar(&name, "name", "", "Plugin name to uninstall")
	pluginCmd.AddCommand(uninstallCmd)

	return pluginCmd
}
