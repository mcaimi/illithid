package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var interceptsCmd = &cobra.Command{
	Use:   "intercepts",
	Short: "Manage interception rules",
}

var interceptsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all interception rules",
	RunE: func(cmd *cobra.Command, args []string) error {
		var intercepts []InterceptLease
		raw, err := apiClient.Get("/api/v1/intercepts", &intercepts)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			fmt.Println(string(raw))
			return nil
		}

		printInterceptTable(intercepts)
		return nil
	},
}

var (
	createMAC       string
	createIP        string
	createGateway   string
	createDNS       []string
	createInterface string
)

var interceptsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new interception rule",
	RunE: func(cmd *cobra.Command, args []string) error {
		payload := map[string]any{
			"mac":       createMAC,
			"ip":        createIP,
			"gateway":   createGateway,
			"dns":       createDNS,
			"interface": createInterface,
		}

		var result InterceptLease
		raw, err := apiClient.Post("/api/v1/intercepts", payload, &result)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			fmt.Println(string(raw))
			return nil
		}

		fmt.Printf("Interception rule created for %s -> %s (gateway: %s)\n", result.MAC, result.IP, result.Gateway)
		return nil
	},
}

var (
	updateIP        string
	updateGateway   string
	updateDNS       []string
	updateInterface string
)

var interceptsUpdateCmd = &cobra.Command{
	Use:   "update <mac>",
	Short: "Update an existing interception rule",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mac := args[0]
		payload := map[string]any{
			"ip":        updateIP,
			"gateway":   updateGateway,
			"dns":       updateDNS,
			"interface": updateInterface,
		}

		var result InterceptLease
		raw, err := apiClient.Put("/api/v1/intercepts/"+mac, payload, &result)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			fmt.Println(string(raw))
			return nil
		}

		fmt.Printf("Interception rule updated for %s -> %s (gateway: %s)\n", result.MAC, result.IP, result.Gateway)
		return nil
	},
}

var interceptsDeleteCmd = &cobra.Command{
	Use:   "delete <mac>",
	Short: "Delete an interception rule",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mac := args[0]
		if err := apiClient.Delete("/api/v1/intercepts/" + mac); err != nil {
			return err
		}
		fmt.Printf("Interception rule for %s deleted.\n", mac)
		return nil
	},
}

func init() {
	interceptsCreateCmd.Flags().StringVar(&createMAC, "mac", "", "Client MAC address (required)")
	interceptsCreateCmd.Flags().StringVar(&createIP, "ip", "", "IP address to assign (required)")
	interceptsCreateCmd.Flags().StringVar(&createGateway, "gateway", "", "Custom gateway address (required)")
	interceptsCreateCmd.Flags().StringSliceVar(&createDNS, "dns", nil, "Custom DNS server(s), comma-separated (required)")
	interceptsCreateCmd.Flags().StringVar(&createInterface, "interface", "", "Target network interface (required)")
	interceptsCreateCmd.MarkFlagRequired("mac")
	interceptsCreateCmd.MarkFlagRequired("ip")
	interceptsCreateCmd.MarkFlagRequired("gateway")
	interceptsCreateCmd.MarkFlagRequired("dns")
	interceptsCreateCmd.MarkFlagRequired("interface")

	interceptsUpdateCmd.Flags().StringVar(&updateIP, "ip", "", "IP address to assign (required)")
	interceptsUpdateCmd.Flags().StringVar(&updateGateway, "gateway", "", "Custom gateway address (required)")
	interceptsUpdateCmd.Flags().StringSliceVar(&updateDNS, "dns", nil, "Custom DNS server(s), comma-separated (required)")
	interceptsUpdateCmd.Flags().StringVar(&updateInterface, "interface", "", "Target network interface (required)")
	interceptsUpdateCmd.MarkFlagRequired("ip")
	interceptsUpdateCmd.MarkFlagRequired("gateway")
	interceptsUpdateCmd.MarkFlagRequired("dns")
	interceptsUpdateCmd.MarkFlagRequired("interface")

	interceptsCmd.AddCommand(interceptsListCmd)
	interceptsCmd.AddCommand(interceptsCreateCmd)
	interceptsCmd.AddCommand(interceptsUpdateCmd)
	interceptsCmd.AddCommand(interceptsDeleteCmd)
}

func printInterceptTable(intercepts []InterceptLease) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "MAC\tIP\tGATEWAY\tDNS\tINTERFACE\tCREATED")
	fmt.Fprintln(w, strings.Repeat("-", 18)+"\t"+strings.Repeat("-", 15)+"\t"+strings.Repeat("-", 15)+"\t"+strings.Repeat("-", 20)+"\t"+strings.Repeat("-", 9)+"\t"+strings.Repeat("-", 20))
	for _, il := range intercepts {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			il.MAC, il.IP, il.Gateway, strings.Join(il.DNS, ","), il.Interface, formatTime(il.CreatedAt))
	}
	w.Flush()
}
