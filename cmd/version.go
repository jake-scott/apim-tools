package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jake-scott/apim-tools/version"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display the version number of the tool",

	RunE: func(cmd *cobra.Command, args []string) error {
		if err := doVersion(); err != nil {
			return err
		}

		return nil
	},
}

func init() {
	versionCmd.Flags().BoolVar(&portalCmdOpts.asJson, "json", false, "Return version as JSON")
	viper.GetViper().BindPFlag("json", versionCmd.Flags().Lookup("json"))

	rootCmd.AddCommand(versionCmd)
}

type versionResult struct {
	Version string `json:"version"`
}

func doVersion() error {
	if viper.GetBool("json") {
		v := versionResult{
			Version: version.Version,
		}

		b, err := json.MarshalIndent(v, "", "    ")
		if err != nil {
			return err
		}

		fmt.Println(string(b))

	} else {
		fmt.Printf("apim-tools version %s\n", version.Version)
	}

	return nil
}
