package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var clientsCmd = &cobra.Command{
	Use:   "clients",
	Short: "View connected DHCP clients",
}

var clientsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all connected clients",
	RunE: func(cmd *cobra.Command, args []string) error {
		var clients []Lease
		raw, err := apiClient.Get("/api/v1/clients", &clients)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			fmt.Println(string(raw))
			return nil
		}

		printLeaseTable(clients)
		return nil
	},
}

func init() {
	clientsCmd.AddCommand(clientsListCmd)
}
