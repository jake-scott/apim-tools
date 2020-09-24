package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"strings"
	"time"

	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/Azure/go-autorest/autorest"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jake-scott/apim-tools/internal/pkg/auth"
	"github.com/jake-scott/apim-tools/internal/pkg/devportal"
	"github.com/jake-scott/apim-tools/internal/pkg/logging"
)

var (
	apimName      string
	backupFile    string
	resourceGroup string
)

var portalBackupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Backup the API Manager Developer Portal",

	RunE: func(cmd *cobra.Command, args []string) error {
		if err := doPortalBackup(); err != nil {
			return err
		}

		return nil
	},
}

func init() {

	portalBackupCmd.Flags().StringVar(&apimName, "apim", "", "API Manager instance")
	portalBackupCmd.Flags().StringVar(&backupFile, "out", "", "Output archive")
	portalBackupCmd.Flags().StringVar(&resourceGroup, "rg", "", "Resource group contianing the APIM instance")

	portalBackupCmd.MarkFlagRequired("apim")
	portalBackupCmd.MarkFlagRequired("out")
	portalBackupCmd.MarkFlagRequired("rg")

	viper.GetViper().BindPFlag("apim", portalBackupCmd.Flags().Lookup("apim"))
	viper.GetViper().BindPFlag("out", portalBackupCmd.Flags().Lookup("out"))
	viper.GetViper().BindPFlag("rg", portalBackupCmd.Flags().Lookup("rg"))

	portalCmd.AddCommand(portalBackupCmd)
}

func doPortalBackup() error {
	// Prepare the oauth bits and pieces
	s := autorest.CreateSender()

	oauthConfig, err := auth.Get().BuildOAuthConfig(azureLoginEndpoint)
	if err != nil {
		return err
	}

	authz, err := auth.Get().GetAuthorizationToken(s, oauthConfig, azureManagementEndpoint)
	if err != nil {
		return err
	}

	// Azure client that decorates the request with API version and access token
	azClient := NewAzureClient(authz, azureApiVersion)

	// Grab the dev portal and management URLs
	logging.Logger().Infof("Querying instance")
	dpUrl, mgmtUrl, err := getInstancelUrls(azClient)
	if err != nil {
		return err
	}
	logging.Logger().Debugf("Dev portal URL: %s, Management API URL: %s", dpUrl, mgmtUrl)

	// Get a SAS token for the Administrator user
	sasToken, err := getSasToken(azClient)
	if err != nil {
		return err
	}

	// APIM client that decorates the request with API version and SAS token
	apimClient := NewApimClient(sasToken, azureApiVersion)

	// Get the BLOB storage URL
	blobUrl, err := getBlobStorageUrl(apimClient, mgmtUrl)
	if err != nil {
		return err
	}

	// Create a ZIP archive
	aw, err := devportal.NewArchiveWriter(viper.GetString("out"))
	if err != nil {
		return err
	}

	// run the backup
	if err := getPortalMetadata(aw, apimClient, mgmtUrl); err != nil {
		return err
	}

	return downloadPortalBlobs(aw, blobUrl)
}

func downloadPortalBlobs(aw *devportal.ArchiveWriter, blobUrlString string) error {
	logging.Logger().Infof("Downloading blobs")

	u, _ := url.Parse(blobUrlString)
	ctx := context.Background()
	containerUrl := azblob.NewContainerURL(*u, azblob.NewPipeline(azblob.NewAnonymousCredential(), azblob.PipelineOptions{}))

	var cOK, cErr int

	defer aw.Close()

	for marker := (azblob.Marker{}); marker.NotDone(); {
		listBlobs, err := containerUrl.ListBlobsFlatSegment(ctx, marker, azblob.ListBlobsSegmentOptions{})
		if err != nil {
			return err
		}

		marker = listBlobs.NextMarker

		for _, blobInfo := range listBlobs.Segment.BlobItems {
			logging.Logger().Debugf("Found blob: %s", blobInfo.Name)

			blobUrl := containerUrl.NewBlobURL(blobInfo.Name)

			if err := aw.AddBlob(blobUrl); err != nil {
				logging.Logger().WithError(err).Errorf("Writing BLOB %s", blobInfo.Name)
				cErr++
			} else {
				cOK++
			}
		}
	}

	logging.Logger().Infof("Got %d blobs, %d errors", cOK, cErr)

	return nil
}

func getPortalMetadata(aw *devportal.ArchiveWriter, cli *ApimClient, mgmtUrl string) error {
	logging.Logger().Infof("Downloading portal metadata")

	// Get content types used by the portal
	contentTypes, err := getContentTypes(cli, mgmtUrl)
	if err != nil {
		return err
	}

	// Get content items for each content type
	var contentItems = make(map[string]interface{})
	for _, ct := range contentTypes {
		subItems, err := getContentItems(cli, mgmtUrl, ct)
		if err != nil {
			return err
		}

		for k, v := range subItems {
			contentItems[k] = v
		}
	}

	// Write data.json
	data, err := json.Marshal(contentItems)
	if err != nil {
		return err
	}

	if err := aw.AddIndex(data); err != nil {
		return err
	}

	return nil
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

// Get a list of supported content types from the portal
func getContentTypes(cli *ApimClient, mgmtUrl string) ([]string, error) {
	reqUrl := fmt.Sprintf("%s/contentTypes", apimMgmtUrl(mgmtUrl))
	resp, err := cli.Get(reqUrl)
	if err != nil {
		return nil, err
	}

	// Only accept HTTP 2xx codes
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Status %s received", resp.Status)
	}

	// Grab the body
	respBody, err := ioutil.ReadAll(resp.Body)

	ctResp := apimPortalContentTypesResponse{}
	if err := json.Unmarshal(respBody, &ctResp); err != nil {
		return nil, err
	}

	// Extract the IDs minus the /contentTypes/ prefix
	types := make([]string, 0, 10)
	for _, ct := range ctResp.Value {
		s := strings.TrimPrefix(ct.Id, "/contentTypes/")
		types = append(types, s)
	}

	logging.Logger().Debugf("Content types: %s", types)
	return types, nil
}

// Get a list of content items for a given content type
func getContentItems(cli *ApimClient, mgmtUrl string, contentType string) (map[string]interface{}, error) {
	reqUrl := fmt.Sprintf("%s/contentTypes/%s/contentItems", apimMgmtUrl(mgmtUrl), contentType)
	resp, err := cli.Get(reqUrl)
	if err != nil {
		return nil, err
	}

	// Only accept HTTP 2xx codes
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Status %s received", resp.Status)
	}

	// Grab the body
	respBody, err := ioutil.ReadAll(resp.Body)

	ciResp := apimPortalContentItemsResponse{}
	if err := json.Unmarshal(respBody, &ciResp); err != nil {
		return nil, err
	}

	// Turn the list into a map keyed on the id field
	types := make(map[string]interface{})
	for _, ct := range ciResp.Value {
		id := ct["id"].(string)

		types[id] = ct
		delete(ct, "id")
	}

	logging.Logger().Debugf("Content items: %+v", types)

	return types, nil
}
