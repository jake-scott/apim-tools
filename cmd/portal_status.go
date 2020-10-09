package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/jake-scott/apim-tools/internal/pkg/logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var portalStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Display the API Manager Developer Portal status",

	RunE: func(cmd *cobra.Command, args []string) error {
		if err := doPortalStatus(); err != nil {
			return err
		}

		return nil
	},
}

func init() {
	portalStatusCmd.Flags().StringVar(&portalCmdOpts.apimName, "apim", "", "API Manager instance")
	portalStatusCmd.Flags().StringVar(&portalCmdOpts.resourceGroup, "rg", "", "Resource group containing the APIM instance")
	portalStatusCmd.Flags().BoolVarP(&portalCmdOpts.asJSON, "json", "j", false, "Return results as JSON")

	errPanic(portalStatusCmd.MarkFlagRequired("apim"))
	errPanic(portalStatusCmd.MarkFlagRequired("rg"))

	errPanic(viper.GetViper().BindPFlag("apim", portalStatusCmd.Flags().Lookup("apim")))
	errPanic(viper.GetViper().BindPFlag("rg", portalStatusCmd.Flags().Lookup("rg")))
	errPanic(viper.GetViper().BindPFlag("json", portalStatusCmd.Flags().Lookup("json")))

	portalCmd.AddCommand(portalStatusCmd)
}

type portalStatusOutput struct {
	IsDeployed  bool   `json:"is_deployed"`
	PublishDate string `json:"portal_version"`
	CodeVersion string `json:"code_version"`
	Version     string `json:"version"`
}

func doPortalStatus() error {
	info, err := buildApimInfo(azureAPIVersion)
	if err != nil {
		return err
	}

	status, err := getDevportalStatus(info.devPortalURL)
	if err != nil {
		return err
	}
	logging.Logger().Debugf("Portal status: %+v", status)

	isDeployed, err := isDevportalDeployed(info.devPortalURL)
	if err != nil {
		return err
	}

	if viper.GetBool("json") {
		var dateStr string
		if status.PortalVersion != (time.Time{}) {
			dateStr = status.PortalVersion.Format(time.RFC3339)
		}

		// Convert to the output format
		ss := portalStatusOutput{
			IsDeployed:  isDeployed,
			CodeVersion: status.CodeVersion,
			Version:     status.Version,
			PublishDate: dateStr,
		}

		b, err := json.MarshalIndent(ss, "", "    ")
		if err != nil {
			return err
		}

		fmt.Println(string(b))
	} else {
		var dateStr string
		if status.PortalVersion != (time.Time{}) {
			dateStr = status.PortalVersion.Local().Format(time.RFC822)
		} else {
			dateStr = "[Not published]"
		}
		fmt.Printf(" Is deployed: %t\n", isDeployed)
		fmt.Printf("Published at: %s\n", dateStr)
		fmt.Printf("Code version: %s\n", status.CodeVersion)
		fmt.Printf("     Version: %s\n", status.Version)
	}

	return nil
}

func parsePublishDate(s string) (t time.Time, err error) {
	if len(s) < 12 {
		return
	}

	yy, err := strconv.Atoi(s[0:4])
	if err != nil {
		return
	}
	mM, err := strconv.Atoi(s[4:6])
	if err != nil {
		return
	}
	dd, err := strconv.Atoi(s[6:8])
	if err != nil {
		return
	}
	hh, err := strconv.Atoi(s[8:10])
	if err != nil {
		return
	}
	mm, err := strconv.Atoi(s[10:12])
	if err != nil {
		return
	}

	return time.Date(yy, time.Month(mM), dd, hh, mm, 0, 0, time.UTC), nil
}
