package cmd

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/traviswt/gke-auth-plugin/pkg/auth"
	"github.com/traviswt/gke-auth-plugin/pkg/conf"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientauthv1 "k8s.io/client-go/pkg/apis/clientauthentication/v1"
	clientauthv1beta1 "k8s.io/client-go/pkg/apis/clientauthentication/v1beta1"
)

// captureStdout redirects os.Stdout while fn runs and returns what was written.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	done := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()

	fn()
	_ = w.Close()
	return <-done
}

// isolateEnv prevents the command from touching real credentials or caches.
func isolateEnv(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("KUBECONFIG", filepath.Join(dir, "config"))
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", filepath.Join(dir, "no-such-credentials.json"))
	t.Setenv("GKE_AUTH_PLUGIN_DO_NOT_CACHE", "")
	t.Setenv("GKE_AUTH_PLUGIN_CLIENT_AUTH_VERSION", "")
	return dir
}

// seedCacheV1 and seedCacheV1Beta1 write a valid, unexpired credential of the
// given version to the cache file next to KUBECONFIG.
func seedCacheV1(t *testing.T, token string) {
	t.Helper()
	exp := metav1.NewTime(time.Now().Add(time.Hour))
	auth.SaveExecCredential(&clientauthv1.ExecCredential{
		TypeMeta: metav1.TypeMeta{APIVersion: clientauthv1.SchemeGroupVersion.Identifier(), Kind: "ExecCredential"},
		Status:   &clientauthv1.ExecCredentialStatus{Token: token, ExpirationTimestamp: &exp},
	})
}

func seedCacheV1Beta1(t *testing.T, token string) {
	t.Helper()
	exp := metav1.NewTime(time.Now().Add(time.Hour))
	auth.SaveExecCredential(&clientauthv1beta1.ExecCredential{
		TypeMeta: metav1.TypeMeta{APIVersion: clientauthv1beta1.SchemeGroupVersion.Identifier(), Kind: "ExecCredential"},
		Status:   &clientauthv1beta1.ExecCredentialStatus{Token: token, ExpirationTimestamp: &exp},
	})
}

func runRoot(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var err error
	out := captureStdout(t, func() {
		err = GetRootCmd(args).Execute()
	})
	return out, err
}

func parseCredential(t *testing.T, out string) (apiVersion, token string) {
	t.Helper()
	var parsed struct {
		APIVersion string `json:"apiVersion"`
		Status     struct {
			Token string `json:"token"`
		} `json:"status"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("output is not valid json: %v\n%s", err, out)
	}
	return parsed.APIVersion, parsed.Status.Token
}

func TestGetRootCmdConfiguration(t *testing.T) {
	c := GetRootCmd(nil)

	if c.Use != conf.BinName {
		t.Errorf("Use = %q, want %q", c.Use, conf.BinName)
	}
	if !c.SilenceUsage || !c.SilenceErrors {
		t.Error("expected SilenceUsage and SilenceErrors to be true")
	}
	if !c.DisableAutoGenTag {
		t.Error("expected DisableAutoGenTag to be true")
	}
	if !c.FParseErrWhitelist.UnknownFlags {
		t.Error("expected unknown flags to be whitelisted")
	}
	if !c.CompletionOptions.DisableDefaultCmd {
		t.Error("expected default completion command to be disabled")
	}
	if c.RunE == nil {
		t.Error("expected RunE to be set")
	}
}

func TestGetRootCmdFlags(t *testing.T) {
	c := GetRootCmd(nil)

	tests := []struct {
		name      string
		shorthand string
		def       string
	}{
		{name: "impersonate_service_account", shorthand: "i", def: ""},
		{name: "client_auth_version", shorthand: "v", def: "v1beta1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := c.Flags().Lookup(tt.name)
			if f == nil {
				t.Fatalf("flag %q not defined", tt.name)
			}
			if f.Shorthand != tt.shorthand {
				t.Errorf("shorthand = %q, want %q", f.Shorthand, tt.shorthand)
			}
			if f.DefValue != tt.def {
				t.Errorf("default = %q, want %q", f.DefValue, tt.def)
			}
		})
	}
}

func TestGetRootCmdSubcommands(t *testing.T) {
	c := GetRootCmd(nil)
	sub, _, err := c.Find([]string{"version"})
	if err != nil {
		t.Fatalf("Find(version) error = %v", err)
	}
	if sub.Name() != "version" {
		t.Errorf("found %q, want version", sub.Name())
	}
	for _, s := range c.Commands() {
		if s.Name() == "completion" {
			t.Error("completion command should not be registered")
		}
	}
}

func TestRootCmdInvalidClientAuthVersion(t *testing.T) {
	for _, args := range [][]string{
		{"-v", "bogus"},
		{"--client_auth_version", "bogus"},
		{"--client_auth_version=bogus"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			isolateEnv(t)
			out, err := runRoot(t, args...)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), "unknown client auth version 'bogus'") {
				t.Errorf("error = %q", err.Error())
			}
			if out != "" {
				t.Errorf("unexpected output %q", out)
			}
		})
	}
}

func TestRootCmdDefaultsToV1Beta1(t *testing.T) {
	isolateEnv(t)
	seedCacheV1Beta1(t, "beta-token")
	out, err := runRoot(t)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	apiVersion, token := parseCredential(t, out)
	if apiVersion != "client.authentication.k8s.io/v1beta1" || token != "beta-token" {
		t.Errorf("got apiVersion=%q token=%q", apiVersion, token)
	}
}

func TestRootCmdClientAuthVersionFlag(t *testing.T) {
	isolateEnv(t)
	seedCacheV1(t, "v1-token")
	out, err := runRoot(t, "-v", "v1")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	apiVersion, token := parseCredential(t, out)
	if apiVersion != "client.authentication.k8s.io/v1" || token != "v1-token" {
		t.Errorf("got apiVersion=%q token=%q", apiVersion, token)
	}
}

func TestRootCmdClientAuthVersionEnvOverride(t *testing.T) {
	t.Run("valid env value overrides flag", func(t *testing.T) {
		isolateEnv(t)
		seedCacheV1(t, "v1-token")
		t.Setenv("GKE_AUTH_PLUGIN_CLIENT_AUTH_VERSION", "v1")
		out, err := runRoot(t, "-v", "bogus")
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		apiVersion, token := parseCredential(t, out)
		if apiVersion != "client.authentication.k8s.io/v1" || token != "v1-token" {
			t.Errorf("got apiVersion=%q token=%q", apiVersion, token)
		}
	})

	t.Run("v1beta1 env value overrides default", func(t *testing.T) {
		isolateEnv(t)
		seedCacheV1Beta1(t, "beta-token")
		t.Setenv("GKE_AUTH_PLUGIN_CLIENT_AUTH_VERSION", "v1beta1")
		out, err := runRoot(t, "--client_auth_version", "v1")
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		apiVersion, _ := parseCredential(t, out)
		if apiVersion != "client.authentication.k8s.io/v1beta1" {
			t.Errorf("got apiVersion=%q", apiVersion)
		}
	})

	for _, val := range []string{"v2", "V1", "bogus", " v1"} {
		t.Run("invalid env value "+val+" is ignored", func(t *testing.T) {
			isolateEnv(t)
			t.Setenv("GKE_AUTH_PLUGIN_CLIENT_AUTH_VERSION", val)
			_, err := runRoot(t, "-v", "flagvalue")
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), "unknown client auth version 'flagvalue'") {
				t.Errorf("error = %q, want flag value to be used", err.Error())
			}
		})
	}
}

func TestRootCmdIgnoresUnknownFlags(t *testing.T) {
	isolateEnv(t)
	seedCacheV1Beta1(t, "beta-token")
	out, err := runRoot(t, "--some-unknown-flag", "--another=1")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if _, token := parseCredential(t, out); token != "beta-token" {
		t.Errorf("token = %q, want beta-token", token)
	}
}

func TestRootCmdPropagatesCredentialErrors(t *testing.T) {
	isolateEnv(t)
	_, err := runRoot(t)
	if err == nil {
		t.Fatal("expected error when no cache and no credentials, got nil")
	}
	if !strings.Contains(err.Error(), "GOOGLE_APPLICATION_CREDENTIALS") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestVersionCmd(t *testing.T) {
	origVersion, origCommit := conf.Version, conf.GitCommit
	t.Cleanup(func() {
		conf.Version, conf.GitCommit = origVersion, origCommit
	})

	t.Run("prints version and commit", func(t *testing.T) {
		conf.Version = "1.2.3"
		conf.GitCommit = "abcdef0"
		out, err := runRoot(t, "version")
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if want := "gke-auth-plugin v1.2.3 gcabcdef0"; out != want {
			t.Errorf("output = %q, want %q", out, want)
		}
	})

	t.Run("prints empty values when not set at build time", func(t *testing.T) {
		conf.Version = ""
		conf.GitCommit = ""
		out, err := runRoot(t, "version")
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if want := "gke-auth-plugin v gc"; out != want {
			t.Errorf("output = %q, want %q", out, want)
		}
	})

	t.Run("does not require credentials", func(t *testing.T) {
		isolateEnv(t)
		conf.Version = "9.9.9"
		conf.GitCommit = "c0ffee"
		out, err := runRoot(t, "version")
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if !strings.HasPrefix(out, conf.BinName+" v9.9.9") {
			t.Errorf("output = %q", out)
		}
	})
}
