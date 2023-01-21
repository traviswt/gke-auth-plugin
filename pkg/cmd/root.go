package cmd

import (
	"github.com/spf13/cobra"
	"github.com/traviswt/gke-auth-plugin/pkg/auth"
	"github.com/traviswt/gke-auth-plugin/pkg/conf"
)

func GetRootCmd(args []string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:               conf.BinName,
		Short:             "GKE Authentication Plugin",
		SilenceUsage:      true,
		DisableAutoGenTag: true,
		SilenceErrors:     true,
		Long:              `GKE Authentication Plugin`,
		RunE: func(c *cobra.Command, args []string) error {
			return auth.Gcp(c.Context())
		},
	}

	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.AddCommand(versionCmd())
	rootCmd.SetArgs(args)

	return rootCmd
}
