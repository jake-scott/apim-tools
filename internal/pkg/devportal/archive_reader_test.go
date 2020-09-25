package devportal

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"testing"
)

const testFile = "/dev/urandom"
const testFileSize = 1000 * 1024

type seekItem struct {
	offset        int64
	whence        int
	expected      int64
	expectSuccess bool
}

func mkRandomData() (*bytes.Buffer, error) {
	rndBuf := new(bytes.Buffer)

	f, err := os.Open(testFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	io.CopyN(rndBuf, f, testFileSize)
	if err != nil {
		return nil, err
	}

	return rndBuf, nil
}

func mkZip() (n uint64, zipData []byte, rndData []byte, err error) {
	zipBuf := new(bytes.Buffer)
	rndBuf, err := mkRandomData()
	if err != nil {
		return
	}
	rndData = rndBuf.Bytes()

	// Create a new zip archive.
	w := zip.NewWriter(zipBuf)

	zf, err := w.Create("test")
	if err != nil {
		return
	}

	nCopied, err := io.CopyN(zf, rndBuf, testFileSize)
	if err != nil {
		return
	}

	// Make sure to check the error on Close.
	if err = w.Close(); err != nil {
		return
	}

	n = uint64(nCopied)
	zipData = zipBuf.Bytes()
	return
}

func TestZipReadSeeker(t *testing.T) {
	// Create a zip with random content
	fsize, zipData, rndData, err := mkZip()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Created zip from %d byte file", fsize)

	tests := []struct {
		seeks             []seekItem
		toRead            int64
		expectReadSuccess bool
		data              []byte
	}{
		{
			[]seekItem{{0, io.SeekStart, 0, true}},
			100, true,
			rndData[0:100],
		},
		{
			[]seekItem{{1234, io.SeekStart, 1234, true}},
			100, true,
			rndData[1234:1334],
		},
		{
			[]seekItem{{-200, io.SeekStart, 0, false}},
			0, false,
			nil,
		},
		{
			[]seekItem{{123, io.SeekStart, 123, true}, {8, io.SeekCurrent, 123 + 8, true}},
			543, true,
			rndData[(123 + 8):(123 + 8 + 543)],
		},
		{
			[]seekItem{{123, io.SeekStart, 123, true}, {-8, io.SeekCurrent, 123 - 8, true}},
			543, true,
			rndData[(123 - 8):(123 - 8 + 543)],
		},
		{
			// Try to seek past the end of the file - *SHOULD WORK* but data read should fail
			[]seekItem{{123, io.SeekCurrent, 123, true}, {2000000, io.SeekCurrent, 123 + 2000000, true}},
			543, false,
			rndData[(123 - 8):(123 - 8 + 543)],
		},
		{
			[]seekItem{
				{123, io.SeekCurrent, 123, true},
				{2000000, io.SeekCurrent, 123 + 2000000, true},        // past the EoF
				{-1000, io.SeekEnd, int64(len(rndData) - 1000), true}, // 1000 bytes before EoF
			},
			500, true,
			rndData[len(rndData)-1000 : len(rndData)-1000+500],
		},
	}

	for _, tt := range tests {
		bytesReader := bytes.NewReader(zipData)

		// New zip reader
		zr, err := zip.NewReader(bytesReader, bytesReader.Size())
		if err != nil {
			t.Fatal(err)
		}

		if len(zr.File) != 1 {
			t.Fatalf("Expected 1 file, got %d", len(zr.File))
		}

		// get first file in archive
		zf := zr.File[0]

		// Reader for the first file in archive
		rc, err := zf.Open()
		if err != nil {
			t.Fatal(err)
		}

		// Wrap the reader in our seekable version
		zrs := ZipReadSeeker{
			ReadCloser: rc,
			f:          zf,
		}

		testBuf := new(bytes.Buffer)

		// Seek around, testing offsets as we go
		var nBadSeeks int
		for _, seek := range tt.seeks {

			off, err := zrs.Seek(seek.offset, seek.whence)
			if seek.expectSuccess {
				if err != nil {
					t.Fatal(err)
				}
				if off != seek.expected {
					t.Errorf("Expected offset %d, got %d for %+v", seek.expected, off, seek)
				}
			} else {
				nBadSeeks++
				if err == nil {
					t.Errorf("Expected error for bad seek, got success %+v", seek)
				}
			}
		}

		// Don't bother comparing contents if the seeks were expected to fail
		if nBadSeeks > 0 {
			continue
		}

		// Read some data and compare to what we expect
		nRead, err := io.CopyN(testBuf, &zrs, tt.toRead)
		if tt.expectReadSuccess {
			if err != nil {
				t.Fatal(err)
			}
			// Check size and content
			if nRead != tt.toRead {
				t.Errorf("Expected %d bytes read, got %d", tt.toRead, nRead)
			}

			if bytes.Compare(tt.data, testBuf.Bytes()) != 0 {
				t.Errorf("data contents mismatch, got %d, wanted %d bytes", nRead, tt.toRead)
			}

		} else {
			if err == nil {
				t.Errorf("Expected read to fail, it succeeed for test %+v", tt)
			}
		}

	}

}
