package main

import (
	"fmt"
	"os"
)

func main() {
	rootCmd := newRootCommand()
	rootCmd.AddCommand(newS3Cmd())

	err := rootCmd.Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
