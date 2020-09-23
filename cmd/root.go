package cmd

import (
	"fmt"
	"os"

	"github.com/mitchellh/go-homedir"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jake-scott/apim-tools/internal/pkg/auth"
	"github.com/jake-scott/apim-tools/internal/pkg/logging"
)

var (
	cfgFile        string
	debug          bool
	subscriptionId string
	clientId       string
	clientSecret   string
	certPath       string
	certPass       string
	tenant         string
)

const (
	azureLoginEndpoint      = "https://login.microsoftonline.com"
	azureManagementEndpoint = "https://management.azure.com"
	azureApiVersion         = "2019-12-01"
	tokenValidityPeriod     = 30 // minutes
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "apim-tools",
	Short: "Azure API Manager tools",

	PersistentPreRunE: doConfigure,
	SilenceUsage:      true,
	SilenceErrors:     true,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.apim-tools.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "enable debugging (default: false)")

	rootCmd.PersistentFlags().StringVar(&subscriptionId, "subscription", "", "Azure subscription ID")
	rootCmd.PersistentFlags().StringVar(&clientId, "client-id", "", "OAuth client ID (AAD App ID)")
	rootCmd.PersistentFlags().StringVar(&clientSecret, "client-secret", "", "OAuth client secret")
	rootCmd.PersistentFlags().StringVar(&certPath, "cert-file", "", "PKCS12 (.pfx) cert/key")
	rootCmd.PersistentFlags().StringVar(&certPass, "cert-password", "", "cert-file passphrase")
	rootCmd.PersistentFlags().StringVar(&tenant, "tenant", "", "Azure tenant name or ID")

	viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))
	viper.BindPFlag("auth.subscription", rootCmd.PersistentFlags().Lookup("subscription"))
	viper.BindPFlag("auth.client-id", rootCmd.PersistentFlags().Lookup("client-id"))
	viper.BindPFlag("auth.client-secret", rootCmd.PersistentFlags().Lookup("client-secret"))
	viper.BindPFlag("auth.cert-file", rootCmd.PersistentFlags().Lookup("cert-file"))
	viper.BindPFlag("auth.cert-pass", rootCmd.PersistentFlags().Lookup("subscription"))
	viper.BindPFlag("auth.tenant", rootCmd.PersistentFlags().Lookup("tenant"))
}

func er(msg interface{}) {
	fmt.Println("Error:", msg)
	os.Exit(1)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			er(err)
		}

		// Search config in home directory with name ".apim-tools" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".apim-tools")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

// Runs before any command handlers - set up logging and auth
func doConfigure(cmd *cobra.Command, args []string) error {
	if viper.GetBool("debug") {
		logrus.SetLevel(logrus.DebugLevel)
	}

	// Initialize logging
	if err := logging.Configure(viper.Sub("logging")); err != nil {
		return err
	}

	if err := auth.Configure(viper.GetViper()); err != nil {
		return err
	}

	return nil
}
