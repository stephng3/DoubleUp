package main

import (
	"./cmd"
	"fmt"
	"os"
)


func main() {
	// See ./cmd/root.go
	err := cmd.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)
		os.Exit(1)
	}
}