package cmd

import (
	"bytes"
	"github.com/spf13/cobra"
	"os"
	"strconv"
	"strings"
	"testing"
)

// Utility functions
func ErrorContains(out error, want string) bool {
	if out == nil {
		return want == ""
	}
	if want == "" {
		return false
	}
	return strings.Contains(out.Error(), want)
}

func executeCommand(root *cobra.Command, args ...string) (output string, err error) {
	_, output, err = executeCommandC(root, args...)
	return output, err
}

func executeCommandC(root *cobra.Command, args ...string) (c *cobra.Command, output string, err error) {
	buf := new(bytes.Buffer)
	root.SetOutput(buf)
	root.SetArgs(args)
	c, err = root.ExecuteC()
	return c, buf.String(), err
}

func checkNoErrorsAndOutputs(t *testing.T, output string, err error) {
	if output != "" {
		t.Errorf("Unexpected output: %v", output)
	}
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

// Setup and teardown for each test case
func TestMain(m *testing.M) {
	rootCmd.RunE = func(_ *cobra.Command, args []string) error { return nil }
	res := m.Run()
	nThreads = 1
	os.Exit(res)
}

// Test valid inputs
func runAndTestValidFlag(t *testing.T, testVal int) {
	testValStr := strconv.Itoa(testVal)
	cmdInputs := [][]string{
		{"http://www.google.com", "-c", testValStr},
		{"-c", testValStr, "http://www.google.com"},
		{"http://www.google.com", "--nThreads", testValStr},
		{"--nThreads", testValStr, "http://www.google.com"},
	}

	for _, inputs := range cmdInputs {
		output, err := executeCommand(rootCmd, inputs...)
		checkNoErrorsAndOutputs(t, output, err)
	}
}

func TestValidThreadsFlag(t *testing.T) {

	// Testing default value
	output, err := executeCommand(rootCmd, "http://www.google.com")
	checkNoErrorsAndOutputs(t, output, err)
	if nThreads != 1 {
		t.Errorf("Default cFlag not equal 1: %d", nThreads)
	}

	for _, c := range []int{1, 4, 100000} {
		runAndTestValidFlag(t, c)
	}
}

func TestValidURL(t *testing.T) {
	cmdInputs := []string{
		"http://www.google.com",
		"https://www.google.com",
		"http://app.google.com",
		"http://www.google.com/foo/bar",
	}
	for _, input := range cmdInputs {
		output, err := executeCommand(rootCmd, input, "-c", "1")
		checkNoErrorsAndOutputs(t, output, err)
	}
}

// Test invalid inputs
func TestInvalidThreadsFlag(t *testing.T) {
	_, err := executeCommand(rootCmd, "http://www.google.com", "-c", "-1")
	if !ErrorContains(err, "nThreads less than 1") {
		t.Error(err)
	}
	err = nil
	_, err = executeCommand(rootCmd, "http://www.google.com", "-c", "0")
	if !ErrorContains(err, "nThreads less than 1") {
		t.Error(err)
	}
	err = nil
	_, err = executeCommand(rootCmd, "http://www.google.com", "-c", "foo")
	if !ErrorContains(err, "invalid argument \"foo\" for \"-c, --nThreads\" flag") {
		t.Error(err)
	}
}

func TestInvalidURL(t *testing.T) {
	_, err := executeCommand(rootCmd, "google.com")
	if !ErrorContains(err, "invalid URI for request") {
		t.Error(err)
	}
	err = nil
	_, err = executeCommand(rootCmd, "127.0.0.1")
	if !ErrorContains(err, "invalid URI for request") {
		t.Error(err)
	}
	err = nil
	_, err = executeCommand(rootCmd, "ftp://google.com")
	if !ErrorContains(err, "invalid URLString string, should be [http|https]://<host>[/path/to/resource]") {
		t.Error(err)
	}
}

func TestInvalidFlags(t *testing.T) {
	cmdInputs := [][]string{
		{"unknown shorthand flag", "http://google.com", "-c", "1", "-d", "2"},
		{"unknown shorthand flag", "http://google.com", "-d", "2"},
		{"unknown flag", "http://google.com", "--cThreads", "2"},
	}

	for _, input := range cmdInputs {
		_, err := executeCommand(rootCmd, input[1:]...)
		if !ErrorContains(err, input[0]) {
			t.Error(err)
		}
	}
}

func TestInvalidArguments(t *testing.T) {
	cmdInputs := [][]string{
		{"too many positional arguments", "http://google.com", "http://facebook.com"},
		{"URL required"},
	}

	for _, input := range cmdInputs {
		_, err := executeCommand(rootCmd, input[1:]...)
		if !ErrorContains(err, input[0]) {
			t.Error(err)
		}
	}
}
