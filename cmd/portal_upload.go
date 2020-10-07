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
	Long: `Uploads a previously downloaded (backed-up) developer portal archive.

By default, media that exists on the portal but is not in the archive, is
deleted from the portal.  This behaviour can be controlled with the --nodelete
option.`,

	RunE: func(cmd *cobra.Command, args []string) error {
		if err := doPortalUpload(); err != nil {
			return err
		}

		return nil
	},
}

func init() {
	portalUploadCmd.Flags().StringVar(&portalCmdOpts.apimName, "apim", "", "API Manager instance")
	portalUploadCmd.Flags().StringVar(&portalCmdOpts.backupFile, "in", "", "Zip archive to upload")
	portalUploadCmd.Flags().StringVar(&portalCmdOpts.resourceGroup, "rg", "", "Resource group containing the APIM instance")
	portalUploadCmd.Flags().BoolVar(&portalCmdOpts.nodelete, "nodelete", false, "Do not delete extraneous media from portal")

	errPanic(portalUploadCmd.MarkFlagRequired("apim"))
	errPanic(portalUploadCmd.MarkFlagRequired("in"))
	errPanic(portalUploadCmd.MarkFlagRequired("rg"))

	errPanic(viper.GetViper().BindPFlag("apim", portalUploadCmd.Flags().Lookup("apim")))
	errPanic(viper.GetViper().BindPFlag("in", portalUploadCmd.Flags().Lookup("in")))
	errPanic(viper.GetViper().BindPFlag("rg", portalUploadCmd.Flags().Lookup("rg")))
	errPanic(viper.GetViper().BindPFlag("nodelete", portalUploadCmd.Flags().Lookup("nodelete")))

	portalCmd.AddCommand(portalUploadCmd)
}

func doPortalUpload() error {
	info, err := buildApimInfo(azureApiVersion)
	if err != nil {
		return err
	}

	// Get a blob container object
	u, _ := url.Parse(info.devPortalBlobStorageUrl)
	containerUrl := azblob.NewContainerURL(*u, azblob.NewPipeline(azblob.NewAnonymousCredential(), azblob.PipelineOptions{}))

	// Keep a list of what we uploaded
	var blobList = make([]string, 0, 100)
	var contentItemList = make([]string, 0, 100)

	// process the archive
	ar, err := devportal.NewArchiveReader(viper.GetString("in"))
	if err != nil {
		return err
	}
	defer ar.Close()

	// Setup the callbacks
	ar = ar.WithBlobHandler(func(name string, f devportal.ZipReadSeeker) error {
		return uploadBlob(&containerUrl, name, f, &blobList)
	}).WithIndexHandler(func(f devportal.ZipReadSeeker) error {
		return uploadContentItems(info.apimClient, info.apimMgmtUrl, f, &contentItemList)
	})

	// Upload the content
	if err := ar.Process(); err != nil {
		return err
	}

	// Delete extra content unless told not to
	if viper.GetBool("nodelete") {
		logging.Logger().Infoln("Not deleting extra content (--nodelete)")
	} else {
		err = deleteExtraBlobs(&containerUrl, blobList)
		err2 := deleteExtraMediaItems(info.apimClient, info.apimMgmtUrl, contentItemList)

		switch {
		case err == nil && err2 != nil:
			err = err2
		case err != nil && err2 != nil:
			err = fmt.Errorf("Deleting: %s AND %s", err, err2)
		}
	}

	return err
}

func deleteExtraMediaItems(cli *ApimClient, mgmtUrl string, mediaList []string) error {
	// Get content types used by the portal
	contentTypes, err := getContentTypes(cli, mgmtUrl)
	if err != nil {
		return err
	}

	// Get content items for each content type
	var allContentIds []string
	for _, ct := range contentTypes {
		subItems, err := getContentItemsAsMap(cli, mgmtUrl, ct)
		if err != nil {
			return err
		}

		for _, item := range subItems {
			allContentIds = append(allContentIds, item["id"].(string))
		}
	}

	logging.Logger().Debugf("Found %d content items on portal", len(allContentIds))

	// Find content items on the portal that were not in the Zip archive
	extraItems := sliceSubtract(toInterfaceSlice(allContentIds), toInterfaceSlice(mediaList))

	// Delete the extras
	var cOK, cErr int
	for _, idI := range extraItems {
		id := idI.(string)

		reqUrl := apimMgmtUrl(mgmtUrl) + id
		req, err := http.NewRequest("DELETE", reqUrl, nil)
		if err != nil {
			return err
		}

		resp, err := cli.Do(req)
		if err != nil {
			cErr++
			logging.Logger().Errorf("Deleting %s: %s", id, err)
			continue
		}

		// Only accept HTTP 2xx codes
		if resp.StatusCode >= 300 {
			cErr++
			logging.Logger().Errorf("Deleting %s: %s", id, resp.Status)
			continue
		}

		cOK++
	}

	logging.Logger().Infof("Deleted %d extra content items, %d errors", cOK, cErr)

	return nil
}

func deleteExtraBlobs(url *azblob.ContainerURL, blobList []string) error {
	ctx := context.Background()
	// Get a list of blobs in the container
	var allBlobs = make([]string, 0, 100)

	for marker := (azblob.Marker{}); marker.NotDone(); {
		listBlobs, err := url.ListBlobsFlatSegment(ctx, marker, azblob.ListBlobsSegmentOptions{})
		if err != nil {
			return err
		}

		marker = listBlobs.NextMarker

		for _, blobInfo := range listBlobs.Segment.BlobItems {
			allBlobs = append(allBlobs, blobInfo.Name)
		}
	}

	logging.Logger().Debugf("Found %d blobs in container", len(allBlobs))

	// Find blobs in the container that were not in the Zip archive
	extraBlobs := sliceSubtract(toInterfaceSlice(allBlobs), toInterfaceSlice(blobList))

	// Delete the extras
	var cOK, cErr int
	for _, blobNameI := range extraBlobs {
		blobName := blobNameI.(string)
		logging.Logger().Debugf("Deleting blob: %s", blobName)
		blobUrl := url.NewBlobURL(blobName)

		_, err := blobUrl.Delete(ctx, azblob.DeleteSnapshotsOptionNone, azblob.BlobAccessConditions{})
		if err != nil {
			logging.Logger().WithError(err).Errorf("Deleting BLOB %s", blobName)
			cErr++
		} else {
			cOK++
		}
	}

	logging.Logger().Infof("Deleted %d extra media blobs, %d errors", cOK, cErr)

	return nil
}

//nolint:interfacer
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
	if err != nil {
		return err
	}

	// Only accept HTTP 2xx codes
	if resp.StatusCode >= 300 {
		return fmt.Errorf("Status %s received", resp.Status)
	}

	logging.Logger().Debugf("Uploaded item %s", id)
	return nil
}

func uploadContentItems(cli *ApimClient, mgmtUrl string, f devportal.ZipReadSeeker, list *[]string) error {
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
		key := item["id"].(string)
		delete(item, "id")

		err := uploadContentItem(cli, mgmtUrl, key, item)
		if err != nil {
			logging.Logger().Errorf("Uploading content item %s: %s", key, err)
			cErr++
		} else {
			*list = append(*list, key)
			cOK++
		}
	}

	logging.Logger().Infof("  -> Total %d items, %d errors", cOK, cErr)

	return nil
}

func uploadBlob(url *azblob.ContainerURL, name string, f devportal.ZipReadSeeker, list *[]string) error {
	logging.Logger().Debugf("Uploading media blob %s", name)
	blobUrl := url.NewBlockBlobURL(name)
	_, err := blobUrl.Upload(context.Background(), &f, azblob.BlobHTTPHeaders{ContentType: "text/plain"}, azblob.Metadata{}, azblob.BlobAccessConditions{})

	if err != nil {
		return err
	}

	*list = append(*list, name)

	return nil
}
