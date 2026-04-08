package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/user/goharness/internal/config"
)

func newAuthCommand() *cobra.Command {
	authCmd := &cobra.Command{Use: "auth", Short: "Manage authentication"}

	authCmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show authentication status",
		RunE: func(_ *cobra.Command, _ []string) error {
			settings, err := config.LoadSettings()
			if err != nil {
				return err
			}
			provider := "anthropic"
			status := "unauthenticated"
			if settings.APIKey != "" {
				status = "authenticated"
			}
			fmt.Printf("Provider: %s\n", provider)
			fmt.Printf("Status:   %s\n", status)
			return nil
		},
	})

	var apiKey string
	loginCmd := &cobra.Command{
		Use:   "login [api_key]",
		Short: "Configure authentication",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) >= 1 {
				apiKey = args[0]
			}
			if apiKey == "" {
				return fmt.Errorf("--api-key/-k is required in non-interactive mode")
			}
			settings, err := config.LoadSettings()
			if err != nil {
				return err
			}
			settings.APIKey = apiKey
			if err := config.SaveSettings(settings); err != nil {
				return err
			}
			fmt.Println("API key saved.")
			return nil
		},
	}
	loginCmd.Flags().StringVarP(&apiKey, "api-key", "k", "", "API key")
	authCmd.AddCommand(loginCmd)

	authCmd.AddCommand(&cobra.Command{
		Use:   "logout",
		Short: "Remove stored authentication",
		RunE: func(_ *cobra.Command, _ []string) error {
			settings, err := config.LoadSettings()
			if err != nil {
				return err
			}
			settings.APIKey = ""
			if err := config.SaveSettings(settings); err != nil {
				return err
			}
			fmt.Println("Authentication cleared.")
			return nil
		},
	})

	return authCmd
}
