package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/jake-scott/apim-tools/internal/pkg/logging"
)

// Cmd line opts
var portalCmdOpts struct {
	apimName      string
	backupFile    string
	resourceGroup string
}

// Info we need for portal operations
type apimInfo struct {
	azClient                *AzureClient
	apimClient              *ApimClient
	apimSasToken            string
	devPortalBlobStorageUrl string
	devPortalUrl            string
	apimMgmtUrl             string
	apiVersion              string
}

func buildApimInfo(apiVersion string) (i *apimInfo, err error) {
	i = &apimInfo{apiVersion: apiVersion}

	// Azure client that decorates the request with API version and access token
	i.azClient, err = NewAzureClient(apiVersion)
	if err != nil {
		return nil, err
	}

	// Grab the dev portal and management URLs
	logging.Logger().Infof("Querying instance")
	i.devPortalUrl, i.apimMgmtUrl, err = getInstancelUrls(i.azClient)
	if err != nil {
		return nil, err
	}
	logging.Logger().Debugf("Dev portal URL: %s, Management API URL: %s", i.devPortalUrl, i.apimMgmtUrl)

	// Get a SAS token for the Administrator user
	i.apimSasToken, err = getSasToken(i.azClient)
	if err != nil {
		return nil, err
	}

	// APIM client that decorates the request with API version and SAS token
	i.apimClient = NewApimClient(i.apimSasToken, apiVersion)

	// Get the BLOB storage URL
	i.devPortalBlobStorageUrl, err = getBlobStorageUrl(i.apimClient, i.apimMgmtUrl)
	if err != nil {
		return nil, err
	}

	return i, nil
}

// Get the dev portal and management API URLs for the instance
func getInstancelUrls(cli *AzureClient) (string, string, error) {
	// Fetch APIM instance details
	resp, err := cli.Get(instanceMgmtUrl())
	if err != nil {
		return "", "", err
	}

	defer resp.Body.Close()

	// Only accept HTTP 2xx codes
	if resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("Status %s received", resp.Status)
	}

	// Grab the body
	respBody, err := ioutil.ReadAll(resp.Body)

	apim := apimDetails{}
	if err := json.Unmarshal(respBody, &apim); err != nil {
		return "", "", err
	}
	logging.Logger().Debugf("APIM: %+v", apim)

	dpUrl := apim.Properties.PortalUrl
	mgmtUrl := apim.Properties.MgmtUrl

	// Use override in hostname config if there is one
	for _, entry := range apim.Properties.HostnameConfigurations {
		switch entry.Type {
		case "DeveloperPortal":
			dpUrl = "https://" + entry.Hostname
		case "Management":
			mgmtUrl = "https://" + entry.Hostname
		}
	}

	return dpUrl, mgmtUrl, nil
}

// Get a Shared Access token for use with the APIM management API
func getSasToken(cli *AzureClient) (string, error) {
	// Request a token valid for 30 minutes
	expTime := time.Now().Add(time.Minute * tokenValidityPeriod)

	tr := apimTokenRequest{
		Propties: apimTokenRequestProperties{
			KeyType: "primary",
			Expiry:  expTime.UTC().Format(time.RFC3339Nano),
		},
	}

	// User 'name' 1 is Administrator
	sasReqUrl := fmt.Sprintf("%s/users/1/token", instanceMgmtUrl())
	resp, err := cli.Post(sasReqUrl, tr)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	// Only accept HTTP 2xx codes
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("Status %s received", resp.Status)
	}

	// Grab the body
	respBody, err := ioutil.ReadAll(resp.Body)

	tokenResp := apimTokenRequestResponse{}
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return "", err
	}

	logging.Logger().Debugf("APIM SAS token: %s", tokenResp.Value)
	return tokenResp.Value, nil
}

// Get the BLOB storage URL for the instance
func getBlobStorageUrl(cli *ApimClient, mgmtUrl string) (string, error) {
	reqUrl := fmt.Sprintf("%s/portalSettings/mediaContent/listSecrets", apimMgmtUrl(mgmtUrl))
	resp, err := cli.Post(reqUrl, nil)
	if err != nil {
		return "", err
	}

	// Only accept HTTP 2xx codes
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("Status %s received", resp.Status)
	}

	// Grab the body
	respBody, err := ioutil.ReadAll(resp.Body)

	secretsResp := apimListSecretsResponse{}
	if err := json.Unmarshal(respBody, &secretsResp); err != nil {
		return "", err
	}

	logging.Logger().Debugf("Blob store SAS URL: %s", secretsResp.Url)
	return secretsResp.Url, nil
}
