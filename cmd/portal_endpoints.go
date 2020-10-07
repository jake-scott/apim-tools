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
	portalEndpointsCmd.Flags().StringVar(&portalCmdOpts.resourceGroup, "rg", "", "Resource group contianing the APIM instance")
	portalEndpointsCmd.Flags().BoolVar(&portalCmdOpts.asJson, "json", false, "Return results as JSON")

	portalEndpointsCmd.MarkFlagRequired("apim")
	portalEndpointsCmd.MarkFlagRequired("rg")

	viper.GetViper().BindPFlag("apim", portalEndpointsCmd.Flags().Lookup("apim"))
	viper.GetViper().BindPFlag("rg", portalEndpointsCmd.Flags().Lookup("rg"))
	viper.GetViper().BindPFlag("json", portalEndpointsCmd.Flags().Lookup("json"))

	portalCmd.AddCommand(portalEndpointsCmd)
}

type endpointsInfo struct {
	DevPortalBlobStorageUrl string `json:"blobStorageUrl"`
	DevPortalUrl            string `json:"devPortalUrl"`
	ApimMgmtUrl             string `json:"managementUrl"`
}

func doPortalEndpoints() error {
	info, err := buildApimInfo(azureApiVersion)
	if err != nil {
		return err
	}

	if viper.GetBool("json") {
		ep := endpointsInfo{
			DevPortalBlobStorageUrl: info.devPortalBlobStorageUrl,
			DevPortalUrl:            info.devPortalUrl,
			ApimMgmtUrl:             info.apimMgmtUrl,
		}

		b, err := json.MarshalIndent(ep, "", "    ")
		if err != nil {
			return err
		}

		fmt.Println(string(b))
	} else {
		fmt.Printf("Developer portal URL: %s\n", info.devPortalUrl)
		fmt.Printf("      Management URL: %s\n", info.apimMgmtUrl)
		fmt.Printf("    Blob storage URL: %s\n", info.devPortalBlobStorageUrl)
	}

	return nil
}
