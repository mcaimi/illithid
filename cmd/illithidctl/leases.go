package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var leasesCmd = &cobra.Command{
	Use:   "leases",
	Short: "Manage DHCP leases",
}

var leasesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all DHCP leases",
	RunE: func(cmd *cobra.Command, args []string) error {
		var leases []Lease
		raw, err := apiClient.Get("/api/v1/leases", &leases)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			fmt.Println(string(raw))
			return nil
		}

		printLeaseTable(leases)
		return nil
	},
}

var leasesDeleteCmd = &cobra.Command{
	Use:   "delete <mac>",
	Short: "Delete a lease and release its IP back to the pool",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mac := args[0]
		if err := apiClient.Delete("/api/v1/leases/" + mac); err != nil {
			return err
		}
		fmt.Printf("Lease %s deleted.\n", mac)
		return nil
	},
}

func init() {
	leasesCmd.AddCommand(leasesListCmd)
	leasesCmd.AddCommand(leasesDeleteCmd)
}

func printLeaseTable(leases []Lease) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "MAC\tIP\tHOSTNAME\tINTERFACE\tINTERCEPTED\tEXPIRES")
	fmt.Fprintln(w, strings.Repeat("-", 18)+"\t"+strings.Repeat("-", 15)+"\t"+strings.Repeat("-", 16)+"\t"+strings.Repeat("-", 9)+"\t"+strings.Repeat("-", 11)+"\t"+strings.Repeat("-", 20))
	for _, l := range leases {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%v\t%s\n",
			l.MAC, l.IP, l.Hostname, l.Interface, l.Intercepted, formatTime(l.ExpiresAt))
	}
	w.Flush()
}

func formatTime(t string) string {
	if len(t) > 19 {
		return t[:19]
	}
	return t
}
