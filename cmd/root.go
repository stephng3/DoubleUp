package cmd

import (
	"errors"
	"github.com/spf13/cobra"
	"github.com/stephng3/DoubleUp/constants"
	"github.com/stephng3/DoubleUp/download"
	"net/url"
)

var (
	// Flags
	nThreads    int
	chunkSize   int64
	maxAttempts int
	resource    *url.URL

	rootCmd = &cobra.Command{
		Use:     "downloader <URL>",
		Example: "downloader http://www.google.com -c 4",
		Short:   "A concurrent downloader written in Go.",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("URL required")
			}
			if len(args) > 1 {
				return errors.New("too many positional arguments")
			}
			// Extract, validate, and set URLString
			_, err := url.ParseRequestURI(args[0])
			if err != nil {
				return err
			}
			resource, err = url.Parse(args[0])
			if err != nil {
				return err
			} else if (resource.Scheme != "http" && resource.Scheme != "https") || resource.Host == "" {
				return errors.New("invalid URLString string, should be [http|https]://<host>[/path/to/resource]")
			}
			return nil
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Validate nThreads
			if nThreads < 1 {
				return errors.New("nThreads less than 1")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			download.Downloader(nThreads, resource, chunkSize, maxAttempts)
			return nil
		},
	}
)

// Execute executes the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.Flags().IntVarP(&nThreads, "nThreads", "c", 1, "Number of concurrent goroutines")
	rootCmd.Flags().Int64VarP(&chunkSize, "chunkSize", "s", constants.DefaultChunkSize, "Size of each range request")
	rootCmd.Flags().IntVarP(&maxAttempts, "maxAttempts", "a", constants.DefaultMaxAttempts, "Max number of retries per chunk")
}
