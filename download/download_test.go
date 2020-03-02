package download

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/stephng3/DoubleUp/constants"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	TestFileSize   = 1000000 // 1mb
	ChunkSize      = constants.DefaultChunkSize
	MaxAttempts    = constants.DefaultMaxAttempts
	FailAt         = TestFileSize / 2
	Addr           = ":13355"
	TestFilePrefix = "downloader"
)

var (
	testFileName string
)

func TestMain(m *testing.M) {
	tf, _, err := setupTest()
	if err != nil {
		log.Fatal(err)
	}
	testFileName = tf.Name()
	res := m.Run()
	_ = tf.Close()
	_ = os.Remove(tf.Name())
	os.Exit(res)
}

func TestServer(t *testing.T) {
	testFile, err := os.Open(testFileName)
	if err != nil {
		t.Error(err)
	}
	resp, err := http.Get(getTestEndpoint("/success"))
	if err != nil {
		t.Error(err)
	}
	defer resp.Body.Close()
	err = compareBytesAll(testFile, resp.Body)
	if err != nil {
		t.Error(err)
	}
}

/*
  Tests for getEndpointCapabilities
*/

func TestGetEndpointCapabilities(t *testing.T) {
	success, err := getTestURL("/success")
	if err != nil {
		t.Error(err)
	}
	noRange, err := getTestURL("/no-range")
	if err != nil {
		t.Error(err)
	}
	failRange, err := getTestURL("/fail-range")
	if err != nil {
		t.Error(err)
	}
	endpointTests := []struct {
		in        *url.URL
		chunkType string
		length    int
		canRange  bool
		error     string
	}{
		{success, "bytes", TestFileSize, true, ""},
		{noRange, "", TestFileSize, false, "endpoint does not support range requests"},
		{failRange, "bytes", TestFileSize, true, ""},
	}
	for _, test := range endpointTests {
		err := checkEndpointResults(test.in, test.chunkType, test.length, test.canRange, test.error)
		if err != nil {
			t.Error(err)
		}
	}
}

/*
  Tests for downloadSingleThreaded
*/
func TestDownloadSingleThreaded(t *testing.T) {
	testFile, err := os.Open(testFileName)
	defer testFile.Close()
	if err != nil {
		t.Error(err)
	}
	downloadTest, err := ioutil.TempFile(os.TempDir(), TestFilePrefix)
	defer downloadTest.Close()
	defer os.Remove(downloadTest.Name())
	url, err := getTestURL("/success")
	if err != nil {
		t.Error(err)
	}
	err = downloadSingleThreaded(url, downloadTest)
	if err != nil {
		t.Error(err)
	}
	downloadTest.Seek(0, 0)
	err = compareBytes(testFile, downloadTest)
	if err != nil {
		t.Error(err)
	}
}

/*
  Tests for downloadParallel
*/
func TestDownloadMultiThreadedSuccess(t *testing.T) {
	testFile, err := os.Open(testFileName)
	if err != nil {
		t.Error(err)
	}
	downloadTest, err := ioutil.TempFile(os.TempDir(), TestFilePrefix)
	url, err := getTestURL("/success")
	if err != nil {
		t.Error(err)
	}
	err = downloadParallel("bytes", TestFileSize, url, downloadTest, 4, ChunkSize, MaxAttempts)
	if err != nil {
		t.Error(err)
	}
	downloadTest.Seek(0, 0)
	err = compareBytes(testFile, downloadTest)
	if err != nil {
		t.Error(err)
	}
}

func TestDownloadMultiThreadedFail(t *testing.T) {
	downloadTest, err := ioutil.TempFile(os.TempDir(), TestFilePrefix)
	url, err := getTestURL("/fail-range")
	if err != nil {
		t.Error(err)
	}
	err = downloadParallel("bytes", TestFileSize, url, downloadTest, 4, ChunkSize, MaxAttempts)
	if !strings.Contains(err.Error(), "too many attempts downloading range") {
		t.Error(err)
	}
}

/*
	Utility functions
*/

// Checking for expected endpoint results
func checkEndpointResults(endpoint *url.URL, expChunkType string, expLength int, expCanRange bool, expErr string) error {
	chunkType, length, canRange, err := getEndpointCapabilities(endpoint)
	if err != nil && expErr != "" && !strings.Contains(err.Error(), expErr) {
		return err
	}
	if chunkType != expChunkType {
		return fmt.Errorf("chunkType different for endpoint %v, expected: %s, actual %s", endpoint, expChunkType, chunkType)
	}
	if length != expLength {
		return fmt.Errorf("length different for endpoint %v, expected: %d, actual %d", endpoint, expLength, length)
	}
	if canRange != expCanRange {
		return fmt.Errorf("canRange different for endpoint %v, expected: %v, actual %v", endpoint, expCanRange, canRange)
	}
	return nil
}

// Comparing content
func compareBytesOffset(a *os.File, b io.Reader, offset int64) error {
	originalOffset, err := a.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}
	defer a.Seek(originalOffset, io.SeekStart)
	_, err = a.Seek(offset, io.SeekStart)
	if err != nil {
		return err
	}
	return compareBytes(a, b)
}

func compareBytesAll(a io.Reader, b io.Reader) error {
	aBytes := bytes.NewBuffer(make([]byte, TestFileSize))
	_, err := io.Copy(aBytes, a)
	if err != nil {
		return err
	}
	bBytes := bytes.NewBuffer(make([]byte, TestFileSize))
	_, err = io.Copy(bBytes, b)
	if err != nil {
		return err
	}
	if bytes.Compare(aBytes.Bytes(), bBytes.Bytes()) != 0 {
		return errors.New("bytes differ")
	}
	return nil
}

func compareBytes(a io.Reader, b io.Reader) error {
	aBytes := make([]byte, ChunkSize)
	bBytes := make([]byte, ChunkSize)
	na, errA := a.Read(aBytes)
	if errA != nil {
		return errA
	}
	nb, errB := b.Read(bBytes)
	if errB != nil {
		return errB
	}
	processed := 0
	for errA != io.EOF && errB != io.EOF {
		if na != nb {
			return errors.New(fmt.Sprintf("different number of bytes read, a: %d b: %d at offset %d",
				na, nb, processed))
		}
		if bytes.Compare(aBytes, bBytes) != 0 {
			return errors.New(fmt.Sprintf("bytes starting at offset %d differ, \n",
				processed))
		}
		processed += na
		na, errA = a.Read(aBytes)
		if errA != nil && errA != io.EOF {
			return errA
		}
		nb, errB = b.Read(bBytes)
		if errB != nil && errB != io.EOF {
			return errB
		}
	}
	if errA == nil {
		errors.New(fmt.Sprintf("a has more bytes at offset %d",
			processed))
	}
	if errB == nil {
		errors.New(fmt.Sprintf("b has more bytes at offset %d",
			processed))
	}
	return nil
}

// URL methods
func getTestURL(path string) (res *url.URL, err error) {
	urlString := getTestEndpoint(path)
	return url.Parse(urlString)
}

func getTestEndpoint(path string) string {
	return fmt.Sprintf("http://127.0.0.1%s%s", Addr, path)
}

// Setting up the test server
// https://stackoverflow.com/questions/39320025/how-to-stop-http-listenandserve
func ListenAndServeWithClose(addr string, handler http.Handler) (io.Closer, error) {
	var (
		listener  net.Listener
		srvCloser io.Closer
		err       error
	)
	srv := &http.Server{Addr: addr, Handler: handler}
	if addr == "" {
		addr = ":http"
	}
	listener, err = net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	go func() {
		err := srv.Serve(tcpKeepAliveListener{listener.(*net.TCPListener)})
		if err != nil {
			log.Println("HTTP Server Error - ", err)
		}
	}()
	srvCloser = listener
	return srvCloser, nil
}

type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(3 * time.Minute)
	return tc, nil
}

/*
This function does the following to set up necessary resources for our E2E testing:
1) Creates a temporary file and fills it with random content
2) Defines http handlers at the following routes:
	a) /no-range - does not support range requests, but writes the content of our temporary file to the client
	b) /success - supports range requests and serves our temporary file properly
	c) /fail-range - supports range requests, but when client requests a range that includes FailAt,
	   responds with a 500 internal server error
3) Starts the http server with the handlers at (2) and listens on localhost:Addr
It is the responsibility of the caller to clean up the setup by removing the tempfile and closing the server.
*/
func setupTest() (testFile *os.File, testServer io.Closer, err error) {
	tmpFile, err := ioutil.TempFile(os.TempDir(), TestFilePrefix)
	log.Println("Created tempfile")
	if err != nil {
		log.Fatal("Cannot create temporary file", err)
	}
	if _, err = tmpFile.Write([]byte(randString(TestFileSize))); err != nil {
		return nil, nil, err
	}
	log.Println("Wrote tempfile contents")
	mux := http.NewServeMux()

	mux.HandleFunc("/no-range", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Add("Content-Length", strconv.Itoa(TestFileSize))
		if request.Method == "HEAD" {
			writer.Header().Add("Content-Type", "text/plain; charset=utf-8")
			writer.WriteHeader(200)
			return
		}
		fd, err := os.Open(tmpFile.Name())
		defer fd.Close()
		if err != nil {
			log.Printf("Error opening test file: \n%v\n", err)
		}
		_, err = io.Copy(writer, fd)
		if err != nil {
			log.Printf("Error writing to response: \n%v\n", err)
		}
	})

	mux.HandleFunc("/success", func(writer http.ResponseWriter, request *http.Request) {
		fd, err := os.Open(tmpFile.Name())
		defer fd.Close()
		if err != nil {
			log.Printf("Error opening test file: \n%v\n", err)
		}
		http.ServeContent(writer, request, tmpFile.Name(), time.Unix(0, 0), fd)
	})

	// Todo: handle multi range requests
	mux.HandleFunc("/fail-range", func(writer http.ResponseWriter, request *http.Request) {
		fd, err := os.Open(tmpFile.Name())
		defer fd.Close()
		if request.Method != "HEAD" {
			requestedRange := request.Header.Get("Range")
			if len(requestedRange) == 0 {
				writer.WriteHeader(400)
				_, err = writer.Write([]byte("not a range request"))
				if err != nil {
					log.Printf("Failed to write to fail-range writer: \n%v\n", err)
				}
			}
			startEnd := make([]int64, 2)
			requestedRange = strings.TrimPrefix(requestedRange, "bytes=")
			for i, numString := range strings.Split(requestedRange, "-") {
				num, err := strconv.ParseInt(numString, 10, 64)
				if err != nil {
					log.Printf("Error parsing range request: \n%v\n", err)
					writer.WriteHeader(500)
					_, _ = writer.Write([]byte("Error parsing range request"))
				}
				startEnd[i] = num
			}
			if FailAt >= startEnd[0] && FailAt <= startEnd[1] {
				writer.WriteHeader(500)
				writer.Write([]byte("Internal server error"))
			} else {
				http.ServeContent(writer, request, tmpFile.Name(), time.Unix(0, 0), fd)
			}
		} else {
			http.ServeContent(writer, request, tmpFile.Name(), time.Unix(0, 0), fd)
		}
	})
	testServer, err = ListenAndServeWithClose(Addr, mux)
	log.Printf("Server listening at %s\n", Addr)
	return tmpFile, testServer, nil
}

// https://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-length-in-go
func randString(n int) string {
	const (
		letterBytes   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
		letterIdxBits = 6                    // 6 bits to represent a letter index
		letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
		letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
	)
	src := rand.NewSource(time.Now().UnixNano())

	sb := strings.Builder{}
	sb.Grow(n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			sb.WriteByte(letterBytes[idx])
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return sb.String()
}
