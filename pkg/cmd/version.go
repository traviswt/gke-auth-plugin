package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/traviswt/gke-auth-plugin/pkg/conf"
)

func versionCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "version",
		Short: "prints the version of this plugin",
		Long:  `prints the version of this plugin`,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _ = fmt.Printf("%s v%s gc%s", conf.BinName, conf.Version, conf.GitCommit)
			return nil
		},
	}
	return c
}
