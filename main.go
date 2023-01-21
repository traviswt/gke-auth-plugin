package main

import (
	"fmt"
	"github.com/traviswt/gke-auth-plugin/pkg/cmd"
	"os"
)

func main() {
	rootCmd := cmd.GetRootCmd(os.Args[1:])
	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("failure during execution: %v", err)
		os.Exit(1)
	}
	os.Exit(0)
}
