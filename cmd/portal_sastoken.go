package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var portalSastokenCmd = &cobra.Command{
	Use:   "sastoken",
	Short: "Vend a Shared Access Signature token for the API Manager Developer Portal",

	RunE: func(cmd *cobra.Command, args []string) error {
		if err := doPortalSastoken(); err != nil {
			return err
		}

		return nil
	},
}

func init() {

	portalSastokenCmd.Flags().StringVar(&portalCmdOpts.apimName, "apim", "", "API Manager instance")
	portalSastokenCmd.Flags().StringVar(&portalCmdOpts.resourceGroup, "rg", "", "Resource group contianing the APIM instance")
	portalSastokenCmd.Flags().BoolVar(&portalCmdOpts.asJson, "json", false, "Return results as JSON")

	portalSastokenCmd.MarkFlagRequired("apim")
	portalSastokenCmd.MarkFlagRequired("rg")

	viper.GetViper().BindPFlag("apim", portalSastokenCmd.Flags().Lookup("apim"))
	viper.GetViper().BindPFlag("rg", portalSastokenCmd.Flags().Lookup("rg"))
	viper.GetViper().BindPFlag("json", portalSastokenCmd.Flags().Lookup("json"))

	portalCmd.AddCommand(portalSastokenCmd)
}

type sastokenInfo struct {
	SasToken string `json:"token"`
}

func doPortalSastoken() error {
	info, err := buildApimInfo(azureApiVersion)
	if err != nil {
		return err
	}

	if viper.GetBool("json") {
		ep := sastokenInfo{
			SasToken: info.apimSasToken,
		}

		b, err := json.MarshalIndent(ep, "", "    ")
		if err != nil {
			return err
		}

		fmt.Println(string(b))
	} else {
		fmt.Println(info.apimSasToken)
	}

	return nil
}
