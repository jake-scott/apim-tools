package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/Azure/go-autorest/autorest"
	"github.com/jake-scott/apim-tools/internal/pkg/logging"
	"github.com/spf13/viper"
)

// Construct the instance ID
func instanceId() string {
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.ApiManagement/service/%s",
		viper.GetString("auth.subscription"),
		viper.GetString("rg"),
		viper.GetString("apim"),
	)
}

func instanceMgmtUrl() string {
	return azureManagementEndpoint + instanceId()
}

func apimMgmtUrl(mgmtHost string) string {
	return mgmtHost + "/subscriptions/00000/resourceGroups/00000/providers/Microsoft.ApiManagement/service/00000"
}

func prepUrl(surl string) (*url.URL, error) {
	// String URL -> Parsed URL
	u, err := url.Parse(surl)
	if err != nil {
		return nil, err
	}

	// Parse the query string
	vals, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return nil, err
	}

	// .. and append the API version
	vals.Add("api-version", azureApiVersion)
	u.RawQuery = vals.Encode()

	return u, nil
}

func azPost(authz autorest.Authorizer, surl string, body interface{}) ([]byte, error) {
	u, err := prepUrl(surl)
	if err != nil {
		return nil, err
	}

	requestBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	// Prepare the request by adding the authz header
	req := &http.Request{Method: "POST", URL: u, Body: ioutil.NopCloser(bytes.NewBuffer(requestBody))}
	r, err := autorest.Prepare(req, authz.WithAuthorization())
	if err != nil {
		return nil, err
	}

	// Fetch the URL
	logging.Logger().Debugf("POSTing: [%s] [%s]", req.URL, requestBody)
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	// Only accept HTTP 2xx codes
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Status %s received", resp.Status)
	}

	// Grab the body
	responseBody, err := ioutil.ReadAll(resp.Body)
	return responseBody, err
}

// Fetch an Azure management URL, adding the REST version and authorization header
func azGet(authz autorest.Authorizer, surl string) ([]byte, error) {
	u, err := prepUrl(surl)
	if err != nil {
		return nil, err
	}

	// Prepare the request by adding the authz header
	req := &http.Request{Method: "GET", URL: u}
	r, err := autorest.Prepare(req, authz.WithAuthorization())
	if err != nil {
		return nil, err
	}

	// Fetch the URL
	logging.Logger().Debugf("Fetching: [%s]", req.URL)
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	// Only accept HTTP 200 codes
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Status %s received", resp.Status)
	}

	// Grab the body
	body, err := ioutil.ReadAll(resp.Body)
	return body, err
}

// Send an APIM management server request, adding the SAS token
func mgmtRequest(method string, sasToken string, surl string, body interface{}) ([]byte, error) {
	u, err := prepUrl(surl)
	if err != nil {
		return nil, err
	}

	var requestBody []byte
	if body != nil {
		requestBody, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}

	// Prepare the request by adding the authz header
	req := &http.Request{Method: method, URL: u, Body: ioutil.NopCloser(bytes.NewBuffer(requestBody))}
	req.Header = make(map[string][]string)
	req.Header.Add("authorization", "SharedAccessSignature "+sasToken)

	// Fetch the URL
	logging.Logger().Debugf("Sending %s request to: [%s]  Body: [%s]", method, req.URL, requestBody)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	// Only accept HTTP 2xx codes
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Status %s received", resp.Status)
	}

	// Grab the body
	responseBody, err := ioutil.ReadAll(resp.Body)
	return responseBody, err
}
