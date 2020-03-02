package main

import (
	"fmt"
	"github.com/stephng3/DoubleUp/cmd"
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
