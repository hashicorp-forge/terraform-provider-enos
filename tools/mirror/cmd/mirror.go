package main

import (
	"fmt"
	"os"
)

func main() {
	rootCmd := newRootCommand()
	rootCmd.AddCommand(newPopulateCommand())
	rootCmd.AddCommand(newPromoteCommand())

	err := rootCmd.Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
