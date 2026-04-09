package cmd

import (
	"github.com/spf13/cobra"
	"github.com/traviswt/gke-auth-plugin/pkg/auth"
	"github.com/traviswt/gke-auth-plugin/pkg/conf"
)

func GetRootCmd(args []string) *cobra.Command {
	var (
		impersonateServiceAccount string
		clientAuthVersion         string
	)
	rootCmd := &cobra.Command{
		Use:               conf.BinName,
		Short:             "GKE Authentication Plugin",
		Long:              `GKE Authentication Plugin`,
		SilenceUsage:      true,
		DisableAutoGenTag: true,
		SilenceErrors:     true,
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
		RunE: func(c *cobra.Command, args []string) error {
			return auth.Gcp(c.Context(), impersonateServiceAccount, clientAuthVersion)
		},
	}

	rootCmd.Flags().StringVarP(&impersonateServiceAccount, "impersonate_service_account", "i", "", "Google Service Account to Impersonate")
	rootCmd.Flags().StringVarP(&clientAuthVersion, "client_auth_version", "v", "v1", "Client Auth Version, can be 'v1beta1' or 'v1'")
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.AddCommand(versionCmd())
	rootCmd.SetArgs(args)

	return rootCmd
}
