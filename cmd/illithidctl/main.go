package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	serverURL    string
	outputFormat string
	apiClient    *APIClient
)

var rootCmd = &cobra.Command{
	Use:   "illithidctl",
	Short: "CLI client for the Illithid DHCP server",
	Long:  "Command-line interface for managing leases, interception rules, and connected clients on an Illithid DHCP server.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		apiClient = NewAPIClient(serverURL)
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&serverURL, "server", "s", "http://localhost:8080", "Illithid API server URL")
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "table", "Output format: table or json")

	rootCmd.AddCommand(leasesCmd)
	rootCmd.AddCommand(interceptsCmd)
	rootCmd.AddCommand(clientsCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
