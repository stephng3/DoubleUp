package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
)

type Arguments struct {
	URL 	url.URL
	Threads   int
}

type OffsetWriter struct {
	io.WriterAt
	offset int64
}

func (dst *OffsetWriter) Write(b []byte) (n int, err error) {
	n, err = dst.WriteAt(b, dst.offset)
	dst.offset += int64(n)
	return
}

var Client *http.Client

func Usage() {
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "%s <URLString> -c nThreads\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Parameters: \n")
	fmt.Fprintf(os.Stderr, "\t<URLString> (string): Valid URLString\n")
	fmt.Fprintf(os.Stderr, "\t-c (int): Number of concurrent downloaders\n")
	fmt.Fprintf(os.Stderr, "\n")
	os.Exit(3)
}

func parseArgs() Arguments {
	var (
		URL string
		threads int
	)
	args := os.Args

	// Extract -c
	if len(args) == 2 {
		URL = args[1]
		threads = 1
	} else if len(args) == 4 && args[2] == "-c" {
		c, err := strconv.Atoi(args[3])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid argument: {-c: %s}\n", args[3])
			Usage()
		}
		URL = args[1]
		threads = c
	} else {
		Usage()
	}

	// Extract and validate URLString
	_, err := url.ParseRequestURI(URL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
	u, err := url.Parse(URL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	} else if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		fmt.Fprintln(os.Stderr, "Invalid URLString string, should be [http|https]://[host]/[path]")
		os.Exit(3)
	}

	return Arguments{
		URL: *u,
		Threads:   threads,
	}
}

func GetEndpointCapabilities(URL url.URL) (chunkType string, length int, canRange bool) {
	header, err := http.Head(URL.String())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(4)
	}
	chunkType = header.Header.Get("Accept-Ranges")
	l, err := strconv.Atoi(header.Header.Get("Content-Length"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(4)
	}
	if len(chunkType) < 1 || chunkType == "none" {
		fmt.Fprintln(os.Stderr, "Endpoint does not support range requests, defaulting to single-threaded mode")
		return "", 0, false
	}
	return chunkType, l, true
}

func DownloadParallel(chunkType string, length int, URL url.URL, w io.WriterAt, c int) {
	var wg sync.WaitGroup
	for i := 0; i < c; i++ {
		start := int64(i * length / c)
		end := int64((i + 1) * length / c)
		ow := &OffsetWriter{
			WriterAt: w,
			offset:   start,
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			DownloadChunk(URL, chunkType, start, end, ow)
		}()
	}
	wg.Wait()
}

func DownloadChunk(URL url.URL, chunkType string, start int64, end int64, w *OffsetWriter) {
	// Build ranged http get request
	header := http.Header{
		"Range": []string{chunkType + "=" + strconv.FormatInt(start, 10) + "-" + strconv.FormatInt(end, 10)},
	}
	req := &http.Request{
		Method:           "GET",
		URL:              &URL,
		Header:           header,
	}
	res, err := Client.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(5)
	}
	// Copy bytes to destination
	written, err := io.CopyN(w, res.Body, end-start)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(5)
	}
	if written != end-start {
		fmt.Fprintln(os.Stderr, "Wrong number of bytes copied")
		os.Exit(5)
	}
}


func Download(URL url.URL, w io.Writer) {
	res, err := http.Get(URL.String())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(5)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(5)
	}
	_, err = io.Copy(w, res.Body)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(5)
	}
}

func main() {
	args := parseArgs()

	// Extracting filename and creating file
	path := strings.Split(args.URL.String(), "/")
	if err := os.MkdirAll("downloads",0777); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(6)
	}
	filename := path[len(path)-1]
	f, err := os.Create("downloads/" + filename)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(6)
	}
	defer f.Close()

	if args.Threads > 1 {
		Client = &http.Client{}
		chunkType, length, canRange := GetEndpointCapabilities(args.URL)
		if canRange {
			DownloadParallel(chunkType, length, args.URL, f, args.Threads)
			os.Exit(0)
		}
	}

	Download(args.URL, f)
}