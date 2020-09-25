package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jake-scott/apim-tools/internal/pkg/devportal"
	"github.com/jake-scott/apim-tools/internal/pkg/logging"
)

var portalUploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "Upload a ZIP archive to the APIM developer portal",
	Long: `Use to upload a previously downloaded (backed-up) developer portal
archive.

NOTE: THIS OPTION WILL ERASE ALL DATA ON THE TARGET PORTAL INSTANCE.`,

	RunE: func(cmd *cobra.Command, args []string) error {
		if err := doPortalUpload(); err != nil {
			return err
		}

		return nil
	},
}

func init() {

	portalUploadCmd.Flags().StringVar(&portalCmdOpts.apimName, "apim", "", "API Manager instance")
	portalUploadCmd.Flags().StringVar(&portalCmdOpts.backupFile, "in", "", "Output archive")
	portalUploadCmd.Flags().StringVar(&portalCmdOpts.resourceGroup, "rg", "", "Resource group contianing the APIM instance")

	portalUploadCmd.MarkFlagRequired("apim")
	portalUploadCmd.MarkFlagRequired("in")
	portalUploadCmd.MarkFlagRequired("rg")

	viper.GetViper().BindPFlag("apim", portalUploadCmd.Flags().Lookup("apim"))
	viper.GetViper().BindPFlag("in", portalUploadCmd.Flags().Lookup("in"))
	viper.GetViper().BindPFlag("rg", portalUploadCmd.Flags().Lookup("rg"))

	portalCmd.AddCommand(portalUploadCmd)
}

func doPortalUpload() error {
	info, err := buildApimInfo(azureApiVersion)
	if err != nil {
		return err
	}

	// Get a container object
	u, _ := url.Parse(info.devPortalBlobStorageUrl)
	containerUrl := azblob.NewContainerURL(*u, azblob.NewPipeline(azblob.NewAnonymousCredential(), azblob.PipelineOptions{}))

	// process the archive
	ar, err := devportal.NewArchiveReader(viper.GetString("in"))
	if err != nil {
		return err
	}
	defer ar.Close()

	// Setup the callbacks
	ar = ar.WithBlobHandler(func(name string, f devportal.ZipReadSeeker) error {
		return uploadBlob(&containerUrl, name, f)
	}).WithIndexHandler(func(f devportal.ZipReadSeeker) error {
		return applyIndex(info.apimClient, info.apimMgmtUrl, f)
	})

	if err := ar.Process(); err != nil {
		return err
	}

	return nil
}

func uploadContentItem(cli *ApimClient, mgmtUrl string, id string, item interface{}) error {
	reqUrl := apimMgmtUrl(mgmtUrl) + id

	var requestBody []byte
	requestBody, err := json.Marshal(item)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", reqUrl, ioutil.NopCloser(bytes.NewBuffer(requestBody)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := cli.Do(req)

	// Only accept HTTP 2xx codes
	if resp.StatusCode >= 300 {
		return fmt.Errorf("Status %s received", resp.Status)
	}

	logging.Logger().Debugf("Uploaded item %s", id)
	return nil
}

func applyIndex(cli *ApimClient, mgmtUrl string, f devportal.ZipReadSeeker) error {
	// Get the index contents
	data, err := ioutil.ReadAll(&f)
	if err != nil {
		return err
	}

	var items []map[string]interface{}
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	logging.Logger().Infof("Processing %d content items", len(items))

	var cOK, cErr int

	// Grab the ID from each item and upload the item
	for _, item := range items {
		key := item["id"]
		delete(item, "id")

		err := uploadContentItem(cli, mgmtUrl, key.(string), item)
		if err != nil {
			logging.Logger().Errorf("Uploading content item %s: %s", key, err)
			cErr++
		} else {
			cOK++
		}
	}

	logging.Logger().Infof("Uploaded %d content items, %d errors", cOK, cErr)

	return nil
}

func uploadBlob(url *azblob.ContainerURL, name string, f devportal.ZipReadSeeker) error {
	blobUrl := url.NewBlockBlobURL(name)
	_, err := blobUrl.Upload(context.Background(), &f, azblob.BlobHTTPHeaders{ContentType: "text/plain"}, azblob.Metadata{}, azblob.BlobAccessConditions{})

	if err != nil {
		return err
	}

	return nil
}
