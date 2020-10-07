package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jake-scott/apim-tools/internal/pkg/logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var portalPublishCmd = &cobra.Command{
	Use:   "publish",
	Short: "Publish the API Manager Developer Portal",

	RunE: func(cmd *cobra.Command, args []string) error {
		if err := doPortalPublish(); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return fmt.Errorf("Publish timed out")
			}

			return err
		}

		return nil
	},
}

func init() {

	portalPublishCmd.Flags().StringVar(&portalCmdOpts.apimName, "apim", "", "API Manager instance")
	portalPublishCmd.Flags().StringVar(&portalCmdOpts.resourceGroup, "rg", "", "Resource group contianing the APIM instance")
	portalPublishCmd.Flags().BoolVarP(&portalCmdOpts.wait, "wait", "w", false, "Wait for completion")

	portalPublishCmd.MarkFlagRequired("apim")
	portalPublishCmd.MarkFlagRequired("rg")

	viper.GetViper().BindPFlag("apim", portalPublishCmd.Flags().Lookup("apim"))
	viper.GetViper().BindPFlag("rg", portalPublishCmd.Flags().Lookup("rg"))
	viper.GetViper().BindPFlag("wait", portalPublishCmd.Flags().Lookup("wait"))

	portalCmd.AddCommand(portalPublishCmd)
}

func doPortalPublish() error {
	info, err := buildApimInfo(azureApiVersion)
	if err != nil {
		return err
	}

	// Get the current publish date
	status1, err := getDevportalStatus(info.devPortalUrl)
	if err != nil {
		return err
	}
	logging.Logger().Debugf("Initial portal status: %+v", status1)

	// If the last publish is not at least a minute ago, wait until it is at least
	// a minute old.  This is necessary because the publish date only has a
	// per-minute resolution
	//
	waitUntil := time.Date(status1.PortalVersion.Year(), status1.PortalVersion.Month(),
		status1.PortalVersion.Day(), status1.PortalVersion.Hour(),
		status1.PortalVersion.Minute(), 0, 0, time.UTC).Add(time.Minute)
	if waitUntil.After(time.Now()) {
		waitFor := waitUntil.Sub(time.Now())
		logging.Logger().Infof("Waiting for %s before publishing portal", waitFor.Truncate(time.Second))
		time.Sleep(waitFor)
	}

	// Trigger the publish
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

	if !viper.GetBool("wait") {
		logging.Logger().Infof("Developer portal publish triggered")
		return nil
	}

	logging.Logger().Infoln("Waiting (max 5 mins) for publish to complete")

	// 5 minute max wait for the portal to be deployed and published
	d := time.Now().Add(time.Minute * 5)
	ctx, cancel := context.WithDeadline(context.Background(), d)
	defer cancel()

	// Loop waiting for initial deployment
	for {
		isDeployed, err := isDevportalDeployedWithContext(ctx, info.devPortalUrl)
		if err != nil {
			return err
		}

		if isDeployed {
			logging.Logger().Debugln("Devportal is deployed")
			break
		}

		logging.Logger().Debugln("Devportal not yet deployed..")
		time.Sleep(5 * time.Second)
	}

	// Wait for the publish date to change
	for {
		status2, err := getDevportalStatusWithContext(ctx, info.devPortalUrl)
		if err != nil {
			return err
		}

		if status1.PortalVersion != status2.PortalVersion {
			logging.Logger().Debugln("Devportal is published")
			break
		}

		logging.Logger().Debugln("Devportal not yet published..")
		time.Sleep(5 * time.Second)
	}

	logging.Logger().Infoln("Developer portal published")
	return nil
}
