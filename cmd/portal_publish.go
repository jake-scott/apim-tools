package cmd

import (
	"fmt"
	"net/http"

	"github.com/jake-scott/apim-tools/internal/pkg/logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var portalPublishCmd = &cobra.Command{
	Use:   "publish",
	Short: "Publish the API Manager Developer Portal",

	RunE: func(cmd *cobra.Command, args []string) error {
		if err := doPortalPublish(); err != nil {
			return err
		}

		return nil
	},
}

func init() {

	portalPublishCmd.Flags().StringVar(&portalCmdOpts.apimName, "apim", "", "API Manager instance")
	portalPublishCmd.Flags().StringVar(&portalCmdOpts.resourceGroup, "rg", "", "Resource group contianing the APIM instance")

	portalPublishCmd.MarkFlagRequired("apim")
	portalPublishCmd.MarkFlagRequired("rg")

	viper.GetViper().BindPFlag("apim", portalPublishCmd.Flags().Lookup("apim"))
	viper.GetViper().BindPFlag("rg", portalPublishCmd.Flags().Lookup("rg"))

	portalCmd.AddCommand(portalPublishCmd)
}

func doPortalPublish() error {
	info, err := buildApimInfo(azureApiVersion)
	if err != nil {
		return err
	}

	reqUrl := fmt.Sprintf("%s/publish", info.devPortalUrl)
	req, err := http.NewRequest("POST", reqUrl, nil)

	if err != nil {
		return err
	}

	resp, err := info.apimClient.Do(req)
	if err != nil {
		return err
	}

	// Only accept HTTP 2xx codes
	if resp.StatusCode >= 300 {
		return fmt.Errorf("Publishing portal, got %s", resp.Status)
	}

	logging.Logger().Infof("Developer portal published")
	return nil
}
