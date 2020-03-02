# Variables
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
BINARY_NAME=downloader

all: deps test build-all

build-all: deps build build-linux build-windows

build:
	mkdir "./build"
	$(GOBUILD) -o ./build/$(BINARY_NAME)

test:
	$(GOTEST) ./...

clean:
	$(GOCLEAN)
	rm -rf ./build

deps:
	$(GOGET) github.com/spf13/cobra
	$(GOGET) github.com/inconshreveable/mousetrap # Windows dependency, include it for cross-compilation

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o ./build/$(BINARY_NAME)_unix_amd64

build-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) -o ./build/$(BINARY_NAME)_windows_amd64