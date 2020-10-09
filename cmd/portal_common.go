package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/jake-scott/apim-tools/internal/pkg/logging"
)

// Cmd line opts
var portalCmdOpts struct {
	apimName      string
	backupFile    string
	resourceGroup string
	force         bool
	nodelete      bool
	asJSON        bool
	wait          bool
}

// Info we need for portal operations
type apimInfo struct {
	azClient                *azureClient
	apimClient              *apimClient
	apimSasToken            string
	devPortalBlobStorageURL string
	devPortalURL            string
	apimMgmtURL             string
	apiVersion              string
}

// Dev portal status
type portalStatusQueryResult struct {
	PortalStatus  int    `json:"Status"`
	PortalVersion string `json:"PortalVersion"`
	CodeVersion   string `json:"CodeVersion"`
	Version       string `json:"Version"`
}

// Normalised version of the portal status
type portalStatusQueryNormalised struct {
	PortalStatus  int
	PortalVersion time.Time
	CodeVersion   string
	Version       string
}

//nolint:unparam
func buildApimInfo(apiVersion string) (i *apimInfo, err error) {
	i = &apimInfo{apiVersion: apiVersion}

	// Azure client that decorates the request with API version and access token
	i.azClient, err = newAzureClient(apiVersion)
	if err != nil {
		return nil, err
	}

	// Grab the dev portal and management URLs
	logging.Logger().Infof("Querying instance")
	i.devPortalURL, i.apimMgmtURL, err = getInstancelURLs(i.azClient)
	if err != nil {
		return nil, err
	}
	logging.Logger().Debugf("Dev portal URL: %s, Management API URL: %s", i.devPortalURL, i.apimMgmtURL)

	// Get a SAS token for the Administrator user
	i.apimSasToken, err = getSasToken(i.azClient)
	if err != nil {
		return nil, err
	}

	// APIM client that decorates the request with API version and SAS token
	i.apimClient = newApimClient(i.apimSasToken, apiVersion)

	// Get the BLOB storage URL
	i.devPortalBlobStorageURL, err = getBlobStorageURL(i.apimClient, i.apimMgmtURL)
	if err != nil {
		return nil, err
	}

	return i, nil
}

// Get the dev portal and management API URLs for the instance
func getInstancelURLs(cli *azureClient) (string, string, error) {
	// Fetch APIM instance details
	resp, err := cli.Get(instanceMgmtURL())
	if err != nil {
		return "", "", err
	}

	defer resp.Body.Close()

	// Only accept HTTP 2xx codes
	if resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("status %s received", resp.Status)
	}

	// Grab the body
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	apim := apimDetails{}
	if err := json.Unmarshal(respBody, &apim); err != nil {
		return "", "", err
	}
	logging.Logger().Debugf("APIM: %+v", apim)

	dpURL := apim.Properties.PortalURL
	mgmtURL := apim.Properties.MgmtURL

	// Use override in hostname config if there is one
	for _, entry := range apim.Properties.HostnameConfigurations {
		switch entry.Type {
		case "DeveloperPortal":
			dpURL = "https://" + entry.Hostname
		case "Management":
			mgmtURL = "https://" + entry.Hostname
		}
	}

	return dpURL, mgmtURL, nil
}

// Get a Shared Access token for use with the APIM management API
func getSasToken(cli *azureClient) (string, error) {
	// Request a token valid for 30 minutes
	expTime := time.Now().Add(time.Minute * tokenValidityPeriod)

	tr := apimTokenRequest{
		Propties: apimTokenRequestProperties{
			KeyType: "primary",
			Expiry:  expTime.UTC().Format(time.RFC3339Nano),
		},
	}

	// User 'name' 1 is Administrator
	sasReqURL := fmt.Sprintf("%s/users/1/token", instanceMgmtURL())
	resp, err := cli.Post(sasReqURL, tr)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	// Only accept HTTP 2xx codes
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("status %s received", resp.Status)
	}

	// Grab the body
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	tokenResp := apimTokenRequestResponse{}
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return "", err
	}

	logging.Logger().Debugf("APIM SAS token: %s", tokenResp.Value)
	return tokenResp.Value, nil
}

// Get the BLOB storage URL for the instance
func getBlobStorageURL(cli *apimClient, mgmtURL string) (string, error) {
	reqURL := fmt.Sprintf("%s/portalSettings/mediaContent/listSecrets", apimMgmtURL(mgmtURL))
	resp, err := cli.Post(reqURL, nil)
	if err != nil {
		return "", err
	}

	// Only accept HTTP 2xx codes
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("status %s received", resp.Status)
	}

	// Grab the body
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	secretsResp := apimListSecretsResponse{}
	if err := json.Unmarshal(respBody, &secretsResp); err != nil {
		return "", err
	}

	logging.Logger().Debugf("Blob store SAS URL: %s", secretsResp.URL)
	return secretsResp.URL, nil
}

// Return slice a with all items in b removed
//
func sliceSubtract(a, b []interface{}) (out []interface{}) {
	bm := make(map[interface{}]bool)
	out = make([]interface{}, 0, len(a))

	for _, v := range b {
		bm[v] = true
	}

	for _, v := range a {
		_, ok := bm[v]

		// If 'a' value is not in 'b' we'll keep it
		if !ok {
			out = append(out, v)
		}
	}

	return
}

func toInterfaceSlice(slice interface{}) (out []interface{}) {
	s := reflect.ValueOf(slice)
	if s.Kind() != reflect.Slice {
		panic("InterfaceSlice() given a non-slice type")
	}

	out = make([]interface{}, s.Len())

	for i := 0; i < s.Len(); i++ {
		out[i] = s.Index(i).Interface()
	}

	return
}

// Get a list of content items for a given content type
func getContentItemsAsMap(cli *apimClient, mgmtURL string, contentType string) ([]map[string]interface{}, error) {
	reqURL := fmt.Sprintf("%s/contentTypes/%s/contentItems", apimMgmtURL(mgmtURL), contentType)
	resp, err := cli.Get(reqURL)
	if err != nil {
		return nil, err
	}

	// Only accept HTTP 2xx codes
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("tatus %s received", resp.Status)
	}

	// Grab the body
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	ciResp := apimPortalContentItemsResponseMap{}
	if err := json.Unmarshal(respBody, &ciResp); err != nil {
		return nil, err
	}

	logging.Logger().Debugf("%d %s items found", len(ciResp.Value), contentType)

	return ciResp.Value, nil
}

// Tests whether the developer portal is deployed or not
func isDevportalDeployed(url string) (bool, error) {
	return isDevportalDeployedWithContext(context.Background(), url)
}

func isDevportalDeployedWithContext(ctx context.Context, url string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == 200:
		return true, nil
	case resp.StatusCode == 404:
		return false, nil
	}

	return false, fmt.Errorf("unknown dev portal status %d (%s)", resp.StatusCode, resp.Status)
}

func getDevportalStatus(dpurl string) (status portalStatusQueryNormalised, err error) {
	return getDevportalStatusWithContext(context.Background(), dpurl)
}

func getDevportalStatusWithContext(ctx context.Context, dpurl string) (status portalStatusQueryNormalised, err error) {
	reqURL := fmt.Sprintf("%s/internal-status-0123456789abcdef", dpurl)
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return
	}

	/*
	 * The dev portal has a bug and often returns a debug HTML response and not
	   the JSON response its meant to, so retry a few times
	*/
	var numRetries int = 3
	var respBody []byte

	for numRetries > 0 {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return status, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			err = fmt.Errorf("portal status: got %s", resp.Status)
			return status, err
		}

		ct := resp.Header.Get("content-type")
		if strings.HasPrefix(ct, "application/json") {
			// Grab the body
			respBody, err = ioutil.ReadAll(resp.Body)
			if err != nil {
				return status, err
			}

			break
		}

		logging.Logger().Warnf("Dev portal returned '%s' response, ignoring", ct)
		time.Sleep(time.Second * 5)
		numRetries--
	}

	if numRetries == 0 {
		err = fmt.Errorf("too many bad responses received, giving up")
		return
	}

	s := portalStatusQueryResult{}
	if err = json.Unmarshal(respBody, &s); err != nil {
		return
	}

	// Normalise the response
	publishDate, err := parsePublishDate(s.PortalVersion)
	if err != nil {
		return
	}

	status.PortalStatus = s.PortalStatus
	status.CodeVersion = s.CodeVersion
	status.Version = s.Version
	status.PortalVersion = publishDate

	logging.Logger().Debugf("Portal status: %+v", status)

	return status, err
}
