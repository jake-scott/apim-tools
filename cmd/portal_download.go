package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"strings"

	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jake-scott/apim-tools/internal/pkg/devportal"
	"github.com/jake-scott/apim-tools/internal/pkg/logging"
)

var portalDownloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download the APIM developer portal content to a ZIP archive",

	RunE: func(cmd *cobra.Command, args []string) error {
		if err := doPortalDownload(); err != nil {
			return err
		}

		return nil
	},
}

func init() {

	portalDownloadCmd.Flags().StringVar(&portalCmdOpts.apimName, "apim", "", "API Manager instance")
	portalDownloadCmd.Flags().StringVar(&portalCmdOpts.backupFile, "out", "", "Output archive")
	portalDownloadCmd.Flags().StringVar(&portalCmdOpts.resourceGroup, "rg", "", "Resource group contianing the APIM instance")

	portalDownloadCmd.MarkFlagRequired("apim")
	portalDownloadCmd.MarkFlagRequired("out")
	portalDownloadCmd.MarkFlagRequired("rg")

	viper.GetViper().BindPFlag("apim", portalDownloadCmd.Flags().Lookup("apim"))
	viper.GetViper().BindPFlag("out", portalDownloadCmd.Flags().Lookup("out"))
	viper.GetViper().BindPFlag("rg", portalDownloadCmd.Flags().Lookup("rg"))

	portalCmd.AddCommand(portalDownloadCmd)
}

func doPortalDownload() error {
	info, err := buildApimInfo(azureApiVersion)
	if err != nil {
		return err
	}

	// Create a ZIP archive
	aw, err := devportal.NewArchiveWriter(viper.GetString("out"))
	if err != nil {
		return err
	}
	defer aw.Close()

	// run the download
	if err := getPortalMetadata(aw, info.apimClient, info.apimMgmtUrl); err != nil {
		return err
	}

	return downloadPortalBlobs(aw, info.devPortalBlobStorageUrl)
}

func downloadPortalBlobs(aw *devportal.ArchiveWriter, blobUrlString string) error {
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
	var contentItems = make([]interface{}, 0, 200)
	for _, ct := range contentTypes {
		subItems, err := getContentItems(cli, mgmtUrl, ct)
		if err != nil {
			return err
		}

		contentItems = append(contentItems, subItems...)
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
func getContentItems(cli *ApimClient, mgmtUrl string, contentType string) ([]interface{}, error) {
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

	logging.Logger().Debugf("%d %s items found", len(ciResp.Value), contentType)

	return ciResp.Value, nil
}
