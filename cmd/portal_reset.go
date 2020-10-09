package cmd

import (
	"context"
	"net/http"
	"net/url"

	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jake-scott/apim-tools/internal/pkg/logging"
)

var portalResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset the APIM developer portal",
	Long: `Delete all deveoper portal contents.
	
NOTE: THIS OPTION IS DESTRUCTIVE AND CANNOT BE REVERSED.`,

	RunE: func(cmd *cobra.Command, args []string) error {
		if err := doPortalReset(); err != nil {
			return err
		}

		return nil
	},
}

func init() {
	portalResetCmd.Flags().StringVar(&portalCmdOpts.apimName, "apim", "", "API Manager instance")
	portalResetCmd.Flags().StringVar(&portalCmdOpts.resourceGroup, "rg", "", "Resource group containing the APIM instance")

	errPanic(portalResetCmd.MarkFlagRequired("apim"))
	errPanic(portalResetCmd.MarkFlagRequired("rg"))

	errPanic(viper.GetViper().BindPFlag("apim", portalResetCmd.Flags().Lookup("apim")))
	errPanic(viper.GetViper().BindPFlag("rg", portalResetCmd.Flags().Lookup("rg")))

	portalCmd.AddCommand(portalResetCmd)
}

func doPortalReset() error {
	info, err := buildApimInfo(azureAPIVersion)
	if err != nil {
		return err
	}

	// run the reset
	if err := deletePortalContentItems(info.apimClient, info.apimMgmtURL); err != nil {
		return err
	}

	return resetPortalBlobs(info.devPortalBlobStorageURL)
}

func deletePortalContentItems(cli *apimClient, mgmtURL string) error {
	logging.Logger().Info("Deleting portal content items")

	// Get content types used by the portal
	contentTypes, err := getContentTypes(cli, mgmtURL)
	if err != nil {
		return err
	}

	// Get content items for each content type
	var contentItems []map[string]interface{}
	for _, ct := range contentTypes {
		subItems, err := getContentItemsAsMap(cli, mgmtURL, ct)
		if err != nil {
			return err
		}

		contentItems = append(contentItems, subItems...)
	}

	var cOK, cErr int

	// Delete the content items
	for _, item := range contentItems {
		id := item["id"].(string)

		reqURL := apimMgmtURL(mgmtURL) + id
		req, err := http.NewRequest("DELETE", reqURL, nil)
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

	logging.Logger().Infof("Deleted %d content items, %d errors", cOK, cErr)

	return nil
}

func resetPortalBlobs(blobURLString string) error {
	logging.Logger().Infof("Deleting blobs")

	u, _ := url.Parse(blobURLString)
	ctx := context.Background()
	containerURL := azblob.NewContainerURL(*u, azblob.NewPipeline(azblob.NewAnonymousCredential(), azblob.PipelineOptions{}))

	var cOK, cErr int

	for marker := (azblob.Marker{}); marker.NotDone(); {
		listBlobs, err := containerURL.ListBlobsFlatSegment(ctx, marker, azblob.ListBlobsSegmentOptions{})
		if err != nil {
			return err
		}

		marker = listBlobs.NextMarker

		for _, blobInfo := range listBlobs.Segment.BlobItems {
			logging.Logger().Debugf("Deleting blob: %s", blobInfo.Name)

			blobURL := containerURL.NewBlobURL(blobInfo.Name)

			_, err = blobURL.Delete(ctx, azblob.DeleteSnapshotsOptionNone, azblob.BlobAccessConditions{})
			if err != nil {
				logging.Logger().WithError(err).Errorf("Deleting BLOB %s", blobInfo.Name)
				cErr++
			} else {
				cOK++
			}
		}
	}

	logging.Logger().Infof("Deleted %d blobs, %d errors", cOK, cErr)

	return nil
}
