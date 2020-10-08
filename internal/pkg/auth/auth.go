package auth

import (
	"fmt"

	"github.com/hashicorp/go-azure-helpers/authentication"
	"github.com/spf13/viper"

	"github.com/jake-scott/apim-tools/internal/pkg/logging"
)

var authCfg *authentication.Config

// Configure application wide Azure authentication
func Configure(v *viper.Viper) error {
	builder := &authentication.Builder{
		SubscriptionID:     v.GetString("auth.subscription"),
		ClientID:           v.GetString("auth.client-id"),
		ClientSecret:       v.GetString("auth.client-secret"),
		TenantID:           v.GetString("auth.tenant"),
		Environment:        v.GetString("auth.environment"),
		MsiEndpoint:        v.GetString("auth.msi-endpoint"),
		ClientCertPassword: v.GetString("auth.cert-password"),
		ClientCertPath:     v.GetString("auth.cert-file"),

		// Feature Toggles
		SupportsClientCertAuth:         true,
		SupportsClientSecretAuth:       true,
		SupportsManagedServiceIdentity: v.GetBool("use-msi"),
		SupportsAzureCliToken:          true,
		SupportsAuxiliaryTenants:       false,
	}

	var err error
	authCfg, err = builder.Build()
	logging.Logger().Debugf("Auth config: %+v", authCfg)

	if err != nil {
		return fmt.Errorf("error building AzureRM Client: %s", err)
	}

	return nil
}

// Get returns the application wide Azure authentication configuration
func Get() *authentication.Config {
	return authCfg
}
