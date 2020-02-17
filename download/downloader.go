package download

import (
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// Chunk represents a HTTP Range Request which has yet to be completed
// goroutines pop off these chunks from a queue
// if the same chunk was reattempted MaxAttempts times,
// abort and report error
type Chunk struct {
	OffsetWriter
	URL *url.URL
	chunkType string
	start int64
	end int64
	attempt int
}

// OffsetWriter allows us to abstract away the problem of piecing together the downloaded chunks
// by letting each goroutine handle seeking to and writing their bytes
type OffsetWriter struct {
	io.WriterAt
	offset int64
}

func (dst *OffsetWriter) Write(b []byte) (n int, err error) {
	n, err = dst.WriteAt(b, dst.offset)
	dst.offset += int64(n)
	return
}

// Initialize global client for reuse
// See https://golang.org/pkg/net/http/#pkg-overview
var Client = &http.Client{}

// Launch a HEAD request to find out endpoint capabilities
func getEndpointCapabilities(URL *url.URL) (chunkType string, length int, canRange bool, err error) {
	header, err := http.Head(URL.String())
	if err != nil {
		return
	}
	chunkType = header.Header.Get("Accept-Ranges")
	length, err = strconv.Atoi(header.Header.Get("Content-Length"))
	if err != nil {
		return
	}
	if len(chunkType) < 1 || chunkType == "none" {
		_, err = fmt.Fprintln(os.Stderr, "Endpoint does not support range requests, defaulting to single-threaded mode")
		return "", 0, false, err
	}
	canRange = true
	return
}

// Concurrent goroutines launching range requests to download pieces of a file
func downloadParallel(chunkType string, length int, URL *url.URL, w io.WriterAt, c int, chunkSize int64, maxAttempts int) error {
	// Initialize tasks and put it into queue
	// Unfortunately we can't close the channel after the initial task generation
	// since failed tasks have a certain number (MaxAttempts) of re-tries before giving up
	chunkChan := make(chan Chunk)
	nTasks := int64(length) / chunkSize + 1
	go func() {
		for i := int64(0); i < nTasks; i++ {
			chunk := Chunk{
				OffsetWriter: OffsetWriter{
					WriterAt: w,
					offset:   i * chunkSize,
				},
				URL:    URL,
				chunkType:    chunkType,
				start:        i * chunkSize,
				end:          int64(math.Min(float64(length), float64((i+1)*chunkSize))),
				attempt:      0,
			}
			chunkChan <- chunk
		}
	}()

	// Make channels for goroutines to report success or failure of individual chunks
	progressChan := make(chan bool)
	errorsChan := make (chan error)

	// Launch c goroutines which pop tasks from queue and downloads the chunks
	// A goroutine that meets an error pushes it into the errorChan
	// whereupon all tasks are popped off the task queue
	// and the main routine reports the error
	for i := 0; i < c; i++ {
		go func() {
			for chunk := range chunkChan{
				if chunk.attempt == maxAttempts {
					errStr := fmt.Sprintf("too many attempts downloading range %d to %d", chunk.start, chunk.end)
					errorsChan <- errors.New(errStr)
					return
				}
				err := downloadChunk(chunk)
				if err != nil {
					// Put chunk back into queue if there was some error in downloading
					_, printErr := fmt.Fprintf(os.Stderr, "\nAttempt %d: Download of range %d-%d %s failed:\n %v\n",
						chunk.attempt + 1, chunk.start, chunk.end, chunk.chunkType, err)
					if printErr != nil {
						errorsChan <- printErr
					}
					chunk.attempt += 1
					chunkChan <- chunk
				} else {
					// Emit success only if chunk successfully downloaded
					progressChan <- true
				}
			}
		}()
	}

	// Display progress for user experience
	_, err := fmt.Fprintf(os.Stdout, "Progress: 0 of %d", nTasks)
	if err != nil {
		return err
	}
	for i := int64(1); i < nTasks + 1; i++ {
		// Fan-in
		select {
		case err := <- errorsChan:
			// Error has occured, close task queue and pop off all remaining tasks
			close(chunkChan)
			for _ = range chunkChan {}
			return err
		case <- progressChan:
			// Consume a progress signal and update progress
			_, err = fmt.Fprintf(os.Stdout, "\rProgress: %d of %d", i, nTasks)
		}
	}
	// Close chunkChan after all chunks have successfully downloaded
	close(chunkChan)
	return nil
}

// A single range request and corresponding write to the OffsetWriter
func downloadChunk(chunk Chunk) error {
	// Build ranged http get request
	header := http.Header{
		"Range": []string{chunk.chunkType + "=" + strconv.FormatInt(chunk.start, 10) + "-" + strconv.FormatInt(chunk.end, 10)},
	}
	req := &http.Request{
		Method:           "GET",
		URL:              chunk.URL,
		Header:           header,
	}
	res, err := Client.Do(req)
	if err != nil {
		return err
	}
	// Copy bytes to destination
	written, err := io.CopyN(&chunk, res.Body, chunk.end-chunk.start)
	if err != nil {
		return err
	}
	if written != chunk.end-chunk.start {
		return errors.New(fmt.Sprintf("wrong number of bytes copied: expected %d, got %d", chunk.end-chunk.start, written))
	}
	return nil
}

// Single threaded downloader
func download(URL *url.URL, w io.Writer) error {
	res, err := http.Get(URL.String())
	defer res.Body.Close()

	if err != nil {
		return err
	}
	_, err = io.CopyN(w, res.Body, res.ContentLength)
	if err != nil {
		return err
	}
	return nil
}

// Driver code to handle various edge cases
func Downloader(nThreads int, resource *url.URL, chunkSize int64, maxAttempts int) error {
	// Downloader saves the result into a file with an escaped name
	// In the future, we can do a <src> <dst> format
	// to allow specification of a destination filename
	f, err := os.Create(strings.Replace(resource.String(), "/", "_", -1))
	if err != nil {
		return err
	}
	defer f.Close()
	fmt.Println(f.Name())

	chunkType , length , canRange , err  := getEndpointCapabilities(resource)
	if err != nil {
		return err
	}

	// Truncate allocates <length> bytes for the file and fills them with empty bytes
	// This allows us to call WriteAt at any position before EOF
	// and write chunks concurrently
	if err := f.Truncate(int64(length)); err != nil {
		return err
	}

	if nThreads == 1 || !canRange {
		// Fall back to single threaded implementation
		err = download(resource, f)
		if err != nil {
			return err
		}
	} else {
		err = downloadParallel(chunkType, length, resource, f, nThreads, chunkSize, maxAttempts)
		if err != nil {
			return err
		}
	}

	return nil
}