## 💪 DoubleUp 💪 - A concurrent downloader written in Go

![travis](https://travis-ci.com/stephng3/DoubleUp.svg?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/stephng3/DoubleUp)](https://goreportcard.com/report/github.com/stephng3/DoubleUp)

![gif](fun.gif)

(Goroutines should work together and not *sabo* (lit. sabotage) each other)
```
Usage:
    downloader <URL> [flags]

Examples:
    downloader http://www.google.com -c 4

Flags:
    -s, --chunkSize int     Size of each range request (default 64000)
    -h, --help              help for downloader
    -a, --maxAttempts int   Max number of retries per chunk (default 5)
    -c, --nThreads int      Number of concurrent goroutines (default 1)
```

## What is this?

DoubleUp launches multiple goroutines, each consuming jobs from a channel-based job queue and launching 
[HTTP Range Requests](https://developer.mozilla.org/en-US/docs/Web/HTTP/Range_requests) to fetch that byte range. 
Synchronisation is handled by go channels and a [neat trick](https://www.reddit.com/r/golang/comments/9ttjb9/how_to_download_single_file_concurrently/e8znoyu?utm_source=share&utm_medium=web2x)
that allows parallel writes to a file in an easy to reason about abstraction.

## Features

- Customizable ChunkSize flag to be adjusted depending on whether downloads are
bound by network bandwidth or filesystem i/o
- Automatic retries of failed range requests up to a threshold so that a single failed request
does not kill all the progress made so far

## Binaries and building from source

Pre-built binaries are available [here](https://github.com/stephng3/DoubleUp/releases). 

To build from source, you need the go toolchain. Follow instructions [here](https://golang.org/doc/install) to install those.

The Makefile has configurations that should suit most needs. Run the following:
```
$ make deps build
```
Your binaries should be in the ./build folder.

## Why Golang?

Golang's amazing concurrency primitives make it an excellent choice for this application. 
Reasoning about channels and the meeting point of different goroutines was much more fun than
worrying about locks and semaphores.

Beyond developer experience, the nature of goroutines means that even if users were to launch millions of them
(please don't), the system is unlikely to fatally freeze.
The Go scheduler takes care of multiplexing them into actual threads, so we get near optimal parallel goroutines for free<sup>TM</sup>.

## What's next? 

- Making a better end-to-end test suite for downloader
- Improving the CLI to support saving to dest_paths
- Maybe adding a "restart from failed attempt" feature (?)

## Etymology

[Double Up](https://www.reddit.com/r/singapore/comments/acp8bz/steamedchickenrice_guide_on_bmt/)
is a colloquial term meaning "hurry up", often used in the Singaporean army. 

While writing the tool, the concurrent goroutines mindlessly picking up tasks from 
the job queue reminded me of NSFs going "man-mode" on large tasks. 
I saw the true meaning of "man-power" in those days
