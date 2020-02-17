package download

import (
	"../constants"
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"testing"
)

/* 	Test file for concurrent downloader
	Current implementation: download a certain resource from a uri
	then check the hash against a precomputed one
	TODO: implement end-to-end http testing using net/http/httptest
 */

const (
	// Go Source
	//uri = "https://dl.google.com/go/go1.13.8.src.tar.gz"
	//sha256ChkSum = "b13bf04633d4d8cf53226ebeaace8d4d2fd07ae6fa676d0844a688339debec34"

	// Samoyed
	uri = "https://cdn.akc.org/akccontentimages/BreedOfficialPortraits/hero/Samoyed.jpg"
	sha256ChkSum = "da8154e446c1b4aff13d46b7c04ff17f07332e3ecafa03e38d89110a9a829d8e"
)

var (
	tmpDirPath string
	goSrcOriginalPath string
	uriParsed *url.URL
)

// Utility function to check hash of downloaded file against precomputed hash
func checkHash(f *os.File) error {
	h := sha256.New()
	h.Reset()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	hash := fmt.Sprintf("%x" , h.Sum(nil))
	if hash != sha256ChkSum {
		return errors.New(fmt.Sprintf("sha256 hashes are different: expected %x, actual %x", sha256ChkSum, hash))
	}
	return nil
}

// Setup and teardown of test
// Downloads file using wget into temporary directory and cleans up all the temporary files afterward
func TestMain (m *testing.M) {
	tmpDirPath, err := ioutil.TempDir("", "downloader_test")
	if err != nil {
		log.Fatal(err)
	}
	tmpDirPath += "/"
	goSrcOriginalPath = tmpDirPath + "original.tmp"
	log.Println("Downloading original...")
	cmd := exec.Command("wget", "-O", goSrcOriginalPath, uri)
	if err = cmd.Run(); err != nil {
		log.Fatal(err)
	}
	log.Println("Download complete")
	f, err := os.Open(goSrcOriginalPath)
	if err != nil {
		log.Fatal(err)
	}
	if err = checkHash(f); err != nil {
		log.Fatal(err)
	}
	uriParsed, err = url.Parse(uri)
	if err != nil {
		log.Fatal(err)
	}
	exit := m.Run()
	os.RemoveAll(tmpDirPath)
	os.Exit(exit)
}

// Test single threaded version of downloader
func TestSingleThreaded(t *testing.T) {
	tmpfile, err := ioutil.TempFile(tmpDirPath, "single_threaded")
	if err != nil {
		t.Error(err)
	}
	t.Logf("Starting download...")
	if err = download(uriParsed, tmpfile); err != nil {
		t.Error(err)
	}
	t.Logf("Download complete")
	tmpfile.Seek(0,io.SeekStart)
	if err = checkHash(tmpfile); err != nil {
		t.Error(err)
	}
}

// Test a single http range request using downloadChunk
func TestChunk(t *testing.T) {
	tmpfile, err := ioutil.TempFile(tmpDirPath, "chunk")
	if err != nil {
		t.Error(err)
	}
	start := int64(1)
	end := int64(16001)
	tmpfile.Truncate(end)
	chunk := Chunk{
		OffsetWriter: OffsetWriter{
			tmpfile,
			1,
		},
		URL:          uriParsed,
		chunkType:    "bytes",
		start:        start,
		end:          end,
		attempt:      0,
	}
	if err = downloadChunk(chunk); err != nil {t.Error(err)}
	tmpfile.Seek(start, io.SeekStart)
	exp, err := os.Open(goSrcOriginalPath)
	if err != nil {t.Error(err)}
	exp.Seek(start, io.SeekStart)
	h := sha256.New()
	io.CopyN(h, tmpfile, end - start)
	resHash := h.Sum(nil)
	h.Reset()
	io.CopyN(h, exp, end-start)
	expHash := h.Sum(nil)
	if bytes.Compare(resHash, expHash) != 0 {
		t.Error(errors.New(fmt.Sprintf("Expected %x, Actual %x\n", expHash, resHash)))
	}
}

// Test downloading an entire file with downloadParallel
func TestMultiThreaded(t *testing.T) {
	tmpfile, err := ioutil.TempFile(tmpDirPath, "multi_threaded")
	if err != nil {t.Error(err)}
	t.Logf("Starting download...")
	chunkType, length, canRange, err := getEndpointCapabilities(uriParsed)
	if err != nil {t.Error(err)}
	if chunkType != "bytes" {t.Error(errors.New(fmt.Sprintf("unexpected chunktype %s\n", chunkType)))}
	if !canRange {t.Error(errors.New("endpoint cannot range"))}
	err = downloadParallel(chunkType, length, uriParsed, tmpfile, runtime.NumCPU(), constants.DefaultChunkSize, constants.DefaultMaxAttempts)
	if err != nil {t.Error(err)}
	t.Logf("Download complete")
	tmpfile.Seek(0,io.SeekStart)
	if err = checkHash(tmpfile); err != nil {
		t.Error(err)
	}
}
