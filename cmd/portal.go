package cmd

import (
	"github.com/spf13/cobra"
)

var portalCmd = &cobra.Command{
	Use:   "devportal",
	Short: "API Manager Developer Portal operations",
}

func init() {
	rootCmd.AddCommand(portalCmd)
}
