package devportal

import (
	"archive/zip"
	"errors"
	"io"
	"io/ioutil"

	"github.com/jake-scott/apim-tools/internal/pkg/logging"
)

type IndexHandler func(f ZipReadSeeker) error
type BlobHandler func(name string, f ZipReadSeeker) error

type ArchiveReader struct {
	reader       *zip.ReadCloser
	indexHandler IndexHandler
	blobHandler  BlobHandler
}

func NewArchiveReader(filename string) (a ArchiveReader, err error) {
	a.reader, err = zip.OpenReader(filename)
	if err != nil {
		return a, err
	}

	return a, nil
}

func (a ArchiveReader) WithIndexHandler(h IndexHandler) ArchiveReader {
	a.indexHandler = h
	return a
}

func (a ArchiveReader) WithBlobHandler(h BlobHandler) ArchiveReader {
	a.blobHandler = h
	return a
}

func (a *ArchiveReader) Close() error {
	return a.reader.Close()
}

func (a *ArchiveReader) Process() error {
	var cOK, cErr, cSkipped int

	for _, f := range a.reader.File {

		// rc can be used to read the content
		rc, err := f.Open()
		if err != nil {
			return err
		}

		zrs := ZipReadSeeker{
			ReadCloser: rc,
			f:          f,
		}

		defer zrs.Close()

		var skipped bool
		switch f.Name {
		case "data.json":
			if a.indexHandler == nil {
				skipped = true
			} else {
				err = a.indexHandler(zrs)
			}
		default:
			if a.blobHandler == nil {
				skipped = true
			} else {
				err = a.blobHandler(f.Name, zrs)
			}
		}

		if err == nil {
			if skipped {
				cSkipped++
			} else {
				cOK++
			}
		} else {
			logging.Logger().WithError(err).Errorf("Handling file %s", f.Name)
			cErr++
		}
	}

	logging.Logger().Infof("Processed %d files, %d skipped, %d errors", cOK, cSkipped, cErr)

	return nil
}

/*
 * The ZIP library does not support seeking within files in the archive, so
 * emulate it by closing the file, re-opening and reading some bytes
 */
type ZipReadSeeker struct {
	io.ReadCloser

	// The underlying ZIP file
	f *zip.File

	// Current offset
	offset uint64
}

func (z *ZipReadSeeker) Read(b []byte) (n int, err error) {
	n, err = z.ReadCloser.Read(b)
	if err == nil {
		z.offset = z.offset + uint64(n)
	}

	logging.Logger().Tracef("ZIP READ: %d bytes, new offset %d", n, z.offset)

	return
}

func (z *ZipReadSeeker) Seek(offset int64, whence int) (absOffset int64, err error) {
	// Calculate the desired absolute offset
	switch whence {
	case io.SeekStart:
		absOffset = offset
	case io.SeekCurrent:
		absOffset = int64(z.offset) + offset
	case io.SeekEnd:
		absOffset = int64(z.f.FileHeader.UncompressedSize64) + offset
	default:
		return 0, errors.New("devportal.ZipReadSeeker.Seek: invalid whence")
	}

	logging.Logger().Tracef("ZIP SEEK: current: %d, offset: %d, whence: %d, new: %d", z.offset, offset, whence, absOffset)

	// cannot seek before BOF
	if absOffset < 0 {
		return 0, errors.New("devportal.ZipReadSeeker.Seek: negative position")
	}

	// Don't do anything if the position wouldn't change
	if uint64(absOffset) == z.offset {
		logging.Logger().Tracef("ZIP SEEK: noop")
		return int64(z.offset), nil
	}

	// Re-open the file
	z.Close()
	z.ReadCloser, err = z.f.Open()
	if err != nil {
		return 0, err
	}
	z.offset = 0

	// Read a bunch of bytes, but only up to the end of the file
	nToRead := absOffset
	if nToRead > int64(z.f.FileHeader.UncompressedSize64) {
		nToRead = int64(z.f.FileHeader.UncompressedSize64)
	}

	_, err = io.CopyN(ioutil.Discard, z, nToRead)
	if err != nil {
		return 0, err
	}
	logging.Logger().Tracef("NEW ZIP OFFSET: %d, %+v", z.offset, err)
	return absOffset, nil
}
