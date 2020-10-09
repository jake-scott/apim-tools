package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var portalEndpointsCmd = &cobra.Command{
	Use:   "endpoints",
	Short: "Display the API Manager Developer Portal endpoints",

	RunE: func(cmd *cobra.Command, args []string) error {
		if err := doPortalEndpoints(); err != nil {
			return err
		}

		return nil
	},
}

func init() {
	portalEndpointsCmd.Flags().StringVar(&portalCmdOpts.apimName, "apim", "", "API Manager instance")
	portalEndpointsCmd.Flags().StringVar(&portalCmdOpts.resourceGroup, "rg", "", "Resource group containing the APIM instance")
	portalEndpointsCmd.Flags().BoolVar(&portalCmdOpts.asJSON, "json", false, "Return results as JSON")

	errPanic(portalEndpointsCmd.MarkFlagRequired("apim"))
	errPanic(portalEndpointsCmd.MarkFlagRequired("rg"))

	errPanic(viper.GetViper().BindPFlag("apim", portalEndpointsCmd.Flags().Lookup("apim")))
	errPanic(viper.GetViper().BindPFlag("rg", portalEndpointsCmd.Flags().Lookup("rg")))
	errPanic(viper.GetViper().BindPFlag("json", portalEndpointsCmd.Flags().Lookup("json")))

	portalCmd.AddCommand(portalEndpointsCmd)
}

type endpointsInfo struct {
	DevPortalBlobStorageURL string `json:"blob_storage_url"`
	DevPortalURL            string `json:"dev_portal_url"`
	ApimMgmtURL             string `json:"management_url"`
}

func doPortalEndpoints() error {
	info, err := buildApimInfo(azureAPIVersion)
	if err != nil {
		return err
	}

	if viper.GetBool("json") {
		ep := endpointsInfo{
			DevPortalBlobStorageURL: info.devPortalBlobStorageURL,
			DevPortalURL:            info.devPortalURL,
			ApimMgmtURL:             info.apimMgmtURL,
		}

		b, err := json.MarshalIndent(ep, "", "    ")
		if err != nil {
			return err
		}

		fmt.Println(string(b))
	} else {
		fmt.Printf("Developer portal URL: %s\n", info.devPortalURL)
		fmt.Printf("      Management URL: %s\n", info.apimMgmtURL)
		fmt.Printf("    Blob storage URL: %s\n", info.devPortalBlobStorageURL)
	}

	return nil
}
