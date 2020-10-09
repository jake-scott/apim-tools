package devportal

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/jake-scott/apim-tools/internal/pkg/logging"
	"github.com/spf13/viper"
)

// ArchiveWriter writes a Zip archive file by processing Blobs from
// an Azure Storage account, and an index (content items)
type ArchiveWriter struct {
	writer     *zip.Writer
	fileHandle *os.File
}

// NewArchiveWriter returns a new ArchiveWriter ready to write
// Blobs and an index to the supplied file
//
// Caller MUST run Close() on the ArchiveWriter or data will be lost
//
func NewArchiveWriter(filename string) (*ArchiveWriter, error) {
	openFlags := os.O_RDWR | os.O_CREATE
	if viper.GetBool("force") {
		openFlags |= os.O_TRUNC
	} else {
		openFlags |= os.O_EXCL
	}

	fh, err := os.OpenFile(filename, openFlags, 0666)
	if err != nil {
		if os.IsExist(err) {
			err = fmt.Errorf("%s.  Use --force to overwrite existing file", err)
		}
		return nil, err
	}

	w := zip.NewWriter(fh)

	return &ArchiveWriter{
		writer:     w,
		fileHandle: fh,
	}, nil
}

// AddBlob copies the Blob from the supplied Azure storage account URL
// to the underlying archive
func (a *ArchiveWriter) AddBlob(url azblob.BlobURL) error {
	// Initiate the Blob download, retrieve some metadata
	dlResponse, err := url.Download(context.Background(), 0, 0, azblob.BlobAccessConditions{}, false)
	if err != nil {
		return err
	}

	// Zip header for this file
	parts := azblob.NewBlobURLParts(url.URL())
	header := zip.FileHeader{
		Name:     parts.BlobName,
		Modified: dlResponse.LastModified(),
	}

	// Write the ZIP header and get a handle to write the contents
	writer, err := a.writer.CreateHeader(&header)
	if err != nil {
		return err
	}

	// Copy the Blob contents to the ZIP
	reader := dlResponse.Body(azblob.RetryReaderOptions{})
	defer reader.Close()

	n, err := io.Copy(writer, reader)
	if err != nil {
		return err
	}

	logging.Logger().Debugf("Wrote %s to ZIP, %d bytes", parts.BlobName, n)

	return nil
}

// AddContentItems writes the content items (index) JSON to the archive
// as data.json
func (a *ArchiveWriter) AddContentItems(data []byte) error {
	// Zip header for this file
	header := zip.FileHeader{
		Name:     "data.json",
		Modified: time.Now(),
	}

	// Write the ZIP header and get a handle to write the contents
	writer, err := a.writer.CreateHeader(&header)
	if err != nil {
		return err
	}

	n, err := writer.Write(data)
	if err != nil {
		return err
	}

	logging.Logger().Debugf("Wrote content items to ZIP, %d bytes", n)

	return nil
}

// Close closes the Zip archive, and MUST be called to prevent data loss
func (a *ArchiveWriter) Close() error {
	if err := a.writer.Close(); err != nil {
		return err
	}

	if err := a.fileHandle.Close(); err != nil {
		return err
	}

	return nil
}
