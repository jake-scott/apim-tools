package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/Azure/go-autorest/autorest"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jake-scott/apim-tools/internal/pkg/auth"
	"github.com/jake-scott/apim-tools/internal/pkg/logging"
)

var (
	apimName      string
	backupDir     string
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
	portalBackupCmd.Flags().StringVar(&backupDir, "dir", "", "Local backup directory")
	portalBackupCmd.Flags().StringVar(&resourceGroup, "rg", "", "Resource group contianing the APIM instance")

	portalBackupCmd.MarkFlagRequired("apim")
	portalBackupCmd.MarkFlagRequired("dir")
	portalBackupCmd.MarkFlagRequired("rg")

	viper.GetViper().BindPFlag("apim", portalBackupCmd.Flags().Lookup("apim"))
	viper.GetViper().BindPFlag("dir", portalBackupCmd.Flags().Lookup("dir"))
	viper.GetViper().BindPFlag("rg", portalBackupCmd.Flags().Lookup("rg"))

	portalCmd.AddCommand(portalBackupCmd)
}

func doPortalBackup() error {
	// Create the backup directory
	if err := os.MkdirAll(viper.GetString("dir"), 0777); err != nil {
		return err
	}

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

	logging.Logger().Infof("Querying instance")
	// Grab the dev portal and management URLs
	dpUrl, mgmtUrl, err := getInstancelUrls(authz)
	if err != nil {
		return err
	}
	logging.Logger().Debugf("Dev portal URL: %s, Management API URL: %s", dpUrl, mgmtUrl)

	// Get a SAS token for the Administrator user
	sasToken, err := getSasToken(authz)
	if err != nil {
		return err
	}

	// Get the BLOB storage URL
	blobUrl, err := getBlobStorageUrl(mgmtUrl, sasToken)
	if err != nil {
		return err
	}
	_ = blobUrl

	// run the backup
	if err := getPortalMetadata(dpUrl, mgmtUrl, sasToken, authz); err != nil {
		return err
	}

	return downloadPortalBlobs(blobUrl)
}

func downloadPortalBlobs(blobUrlString string) error {
	logging.Logger().Infof("Downloading blobs")

	u, _ := url.Parse(blobUrlString)
	ctx := context.Background()
	containerUrl := azblob.NewContainerURL(*u, azblob.NewPipeline(azblob.NewAnonymousCredential(), azblob.PipelineOptions{}))

	var cOK, cErr int

	for marker := (azblob.Marker{}); marker.NotDone(); {
		listBlobs, err := containerUrl.ListBlobsFlatSegment(ctx, marker, azblob.ListBlobsSegmentOptions{})
		if err != nil {
			return err
		}

		marker = listBlobs.NextMarker

		for _, blobInfo := range listBlobs.Segment.BlobItems {
			logging.Logger().Debugf("Found blob: %s", blobInfo.Name)

			blobUrl := containerUrl.NewBlobURL(blobInfo.Name)

			fname := filepath.Join(viper.GetString("dir"), blobInfo.Name)
			fh, err := os.OpenFile(fname, os.O_RDWR|os.O_CREATE, 0666)
			if err != nil {
				cErr++
				logging.Logger().WithError(err).Errorf("Downloading %s", blobInfo.Name)
				continue
			}

			defer fh.Close()

			err = azblob.DownloadBlobToFile(ctx, blobUrl, 0, 0, fh, azblob.DownloadFromBlobOptions{})
			if err != nil {
				cErr++
				logging.Logger().WithError(err).Errorf("Downloading %s", blobInfo.Name)
				continue
			}

			cOK++
			logging.Logger().Infof("Downloaded %s", blobInfo.Name)
		}
	}

	logging.Logger().Infof("Got %d blobs, %d errors", cOK, cErr)

	return nil
}

func getPortalMetadata(dpUrl string, mgmtUrl string, token string, authz autorest.Authorizer) error {
	logging.Logger().Infof("Downloading portal metadata")

	// Get content types used by the portal
	contentTypes, err := getContentTypes(mgmtUrl, token)
	if err != nil {
		return err
	}

	// Get content items for each content type
	var contentItems = make(map[string]interface{})
	for _, ct := range contentTypes {
		subItems, err := getContentItems(mgmtUrl, token, ct)
		if err != nil {
			return err
		}

		for k, v := range subItems {
			contentItems[k] = v
		}
	}

	// Write data.json
	fname := filepath.Join(viper.GetString("dir"), "data.json")
	data, err := json.Marshal(contentItems)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(fname, data, 0644); err != nil {
		return err
	}

	logging.Logger().Infof("Wrote metadata file: %s", fname)

	return nil
}

// Get the dev portal and management API URLs for the instance
func getInstancelUrls(authz autorest.Authorizer) (string, string, error) {
	// Fetch APIM instance details
	resp, err := azGet(authz, instanceMgmtUrl())
	if err != nil {
		return "", "", err
	}

	apim := apimDetails{}
	if err := json.Unmarshal(resp, &apim); err != nil {
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
func getSasToken(authz autorest.Authorizer) (string, error) {
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
	resp, err := azPost(authz, sasReqUrl, tr)
	if err != nil {
		return "", err
	}

	tokenResp := apimTokenRequestResponse{}
	if err := json.Unmarshal(resp, &tokenResp); err != nil {
		return "", err
	}

	logging.Logger().Debugf("APIM SAS token: %s", tokenResp.Value)
	return tokenResp.Value, nil
}

// Get the BLOB storage URL for the instance
func getBlobStorageUrl(mgmtUrl string, token string) (string, error) {
	reqUrl := fmt.Sprintf("%s/portalSettings/mediaContent/listSecrets", apimMgmtUrl(mgmtUrl))
	resp, err := mgmtRequest("POST", token, reqUrl, nil)
	if err != nil {
		return "", err
	}

	secretsResp := apimListSecretsResponse{}
	if err := json.Unmarshal(resp, &secretsResp); err != nil {
		return "", err
	}

	logging.Logger().Debugf("Blob store SAS URL: %s", secretsResp.Url)
	return secretsResp.Url, nil
}

// Get a list of supported content types from the portal
func getContentTypes(mgmtUrl string, token string) ([]string, error) {
	reqUrl := fmt.Sprintf("%s/contentTypes", apimMgmtUrl(mgmtUrl))
	resp, err := mgmtRequest("GET", token, reqUrl, nil)
	if err != nil {
		return nil, err
	}

	ctResp := apimPortalContentTypesResponse{}
	if err := json.Unmarshal(resp, &ctResp); err != nil {
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
func getContentItems(mgmtUrl string, token string, contentType string) (map[string]interface{}, error) {
	reqUrl := fmt.Sprintf("%s/contentTypes/%s/contentItems", apimMgmtUrl(mgmtUrl), contentType)
	resp, err := mgmtRequest("GET", token, reqUrl, nil)
	if err != nil {
		return nil, err
	}

	ciResp := apimPortalContentItemsResponse{}
	if err := json.Unmarshal(resp, &ciResp); err != nil {
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
