package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/Azure/go-autorest/autorest"
	"github.com/jake-scott/apim-tools/internal/pkg/auth"
	"github.com/jake-scott/apim-tools/internal/pkg/logging"
	"github.com/spf13/viper"
)

// Construct the instance ID
func instanceID() string {
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.ApiManagement/service/%s",
		viper.GetString("auth.subscription"),
		viper.GetString("rg"),
		viper.GetString("apim"),
	)
}

func instanceMgmtURL() string {
	return azureManagementEndpoint + instanceID()
}

func apimMgmtURL(mgmtHost string) string {
	return mgmtHost + "/subscriptions/00000/resourceGroups/00000/providers/Microsoft.ApiManagement/service/00000"
}

type apimClient struct {
	http.Client

	sasToken   string
	apiVersion string
}

func newApimClient(sasToken, apiVersion string) *apimClient {
	return &apimClient{
		sasToken:   sasToken,
		apiVersion: apiVersion,
	}
}

func (c *apimClient) GetClient() *http.Client {
	return &c.Client
}

func (c *apimClient) Do(req *http.Request) (*http.Response, error) {
	/* Tack the API Version number to the query string */
	vals, err := url.ParseQuery(req.URL.RawQuery)
	if err != nil {
		return nil, err
	}

	vals.Add("api-version", azureAPIVersion)
	req.URL.RawQuery = vals.Encode()

	/* Decorate the request with he SAS token */
	req.Header.Add("authorization", "SharedAccessSignature "+c.sasToken)

	resp, err := c.Client.Do(req)
	if err == nil {
		logging.Logger().Debugf("[APIM MgmtApi] %s to %s: %s", req.Method, req.URL, resp.Status)
	} else {
		logging.Logger().WithError(err).Errorf("[APIM MgmtApi] %s to %s", req.Method, req.URL)
	}

	return resp, err
}

func (c *apimClient) Get(url string) (resp *http.Response, err error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

func (c *apimClient) Post(url string, body interface{}) (resp *http.Response, err error) {
	var requestBody []byte
	if body != nil {
		requestBody, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequest("POST", url, ioutil.NopCloser(bytes.NewBuffer(requestBody)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	return c.Do(req)
}

type azureClient struct {
	http.Client

	authz      autorest.Authorizer
	apiVersion string
}

func newAzureClient(apiVersion string) (*azureClient, error) {
	// Prepare the oauth bits and pieces
	s := autorest.CreateSender()

	oauthConfig, err := auth.Get().BuildOAuthConfig(azureLoginEndpoint)
	if err != nil {
		return nil, err
	}

	authz, err := auth.Get().GetAuthorizationToken(s, oauthConfig, azureManagementEndpoint)
	if err != nil {
		return nil, err
	}

	return &azureClient{
		authz:      authz,
		apiVersion: apiVersion,
	}, nil
}

func (c *azureClient) GetClient() *http.Client {
	return &c.Client
}

func (c *azureClient) Do(req *http.Request) (*http.Response, error) {
	/* Tack the API Version number to the query string */
	vals, err := url.ParseQuery(req.URL.RawQuery)
	if err != nil {
		return nil, err
	}

	vals.Add("api-version", azureAPIVersion)
	req.URL.RawQuery = vals.Encode()

	/* Decorate the request with he authorizer */
	r, err := autorest.Prepare(req, c.authz.WithAuthorization())
	if err != nil {
		return nil, err
	}

	resp, err := c.Client.Do(r)
	if err == nil {
		logging.Logger().Debugf("[AZ MgmtAPI] %s to %s: %s", req.Method, req.URL, resp.Status)
	} else {
		logging.Logger().WithError(err).Errorf("[AZ MgmtAPI] %s to %s", req.Method, req.URL)
	}

	return resp, err
}

func (c *azureClient) Get(url string) (resp *http.Response, err error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

func (c *azureClient) Post(url string, body interface{}) (resp *http.Response, err error) {
	var requestBody []byte
	if body != nil {
		requestBody, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequest("POST", url, ioutil.NopCloser(bytes.NewBuffer(requestBody)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	return c.Do(req)
}
