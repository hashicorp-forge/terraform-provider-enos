package main

import (
	"fmt"
	"os"
)

func main() {
	rootCmd := newRootCommand()
	rootCmd.AddCommand(newTFCCmd())

	err := rootCmd.Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
