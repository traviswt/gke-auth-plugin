package auth

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// isolateGcp makes sure Gcp never reaches real Google credentials: if the
// cache is missed, application default credentials fail fast.
func isolateGcp(t *testing.T) {
	t.Helper()
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", filepath.Join(t.TempDir(), "no-such-credentials.json"))
	t.Setenv("GKE_AUTH_PLUGIN_DO_NOT_CACHE", "")
}

func TestTrimTrailingPeriods(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  string
	}{
		{name: "no periods", token: "abc123", want: "abc123"},
		{name: "single trailing period", token: "abc123.", want: "abc123"},
		{name: "multiple trailing periods", token: "abc123....", want: "abc123"},
		{name: "inner periods kept", token: "a.b.c", want: "a.b.c"},
		{name: "inner and trailing periods", token: "a.b.c..", want: "a.b.c"},
		{name: "leading periods kept", token: "..abc", want: "..abc"},
		{name: "only periods", token: "...", want: ""},
		{name: "empty", token: "", want: ""},
		{name: "trailing whitespace not trimmed", token: "abc. ", want: "abc. "},
	}
	exp := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newExecCredentialV1Beta1(tt.token, exp).Status.Token; got != tt.want {
				t.Errorf("v1beta1 token = %q, want %q", got, tt.want)
			}
			if got := newExecCredentialV1(tt.token, exp).Status.Token; got != tt.want {
				t.Errorf("v1 token = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewExecCredentialV1Beta1(t *testing.T) {
	exp := time.Date(2030, 6, 7, 8, 9, 10, 0, time.UTC)
	ec := newExecCredentialV1Beta1("token", exp)

	if ec.APIVersion != "client.authentication.k8s.io/v1beta1" {
		t.Errorf("APIVersion = %q", ec.APIVersion)
	}
	if ec.APIVersion != clientauthv1beta1.SchemeGroupVersion.Identifier() {
		t.Errorf("APIVersion = %q, want %q", ec.APIVersion, clientauthv1beta1.SchemeGroupVersion.Identifier())
	}
	if ec.Kind != "ExecCredential" {
		t.Errorf("Kind = %q, want ExecCredential", ec.Kind)
	}
	if ec.Spec.Interactive || ec.Spec.Cluster != nil {
		t.Errorf("Spec = %+v, want zero value", ec.Spec)
	}
	if ec.Status == nil {
		t.Fatal("Status is nil")
	}
	if ec.Status.Token != "token" {
		t.Errorf("Token = %q, want token", ec.Status.Token)
	}
	if ec.Status.ExpirationTimestamp == nil || !ec.Status.ExpirationTimestamp.Time.Equal(exp) {
		t.Errorf("ExpirationTimestamp = %v, want %v", ec.Status.ExpirationTimestamp, exp)
	}
	if ec.Status.ClientCertificateData != "" || ec.Status.ClientKeyData != "" {
		t.Error("client certificate data should be empty")
	}
}

func TestNewExecCredentialV1(t *testing.T) {
	exp := time.Date(2030, 6, 7, 8, 9, 10, 0, time.UTC)
	ec := newExecCredentialV1("token", exp)

	if ec.APIVersion != "client.authentication.k8s.io/v1" {
		t.Errorf("APIVersion = %q", ec.APIVersion)
	}
	if ec.APIVersion != clientauthv1.SchemeGroupVersion.Identifier() {
		t.Errorf("APIVersion = %q, want %q", ec.APIVersion, clientauthv1.SchemeGroupVersion.Identifier())
	}
	if ec.Kind != "ExecCredential" {
		t.Errorf("Kind = %q, want ExecCredential", ec.Kind)
	}
	if ec.Status == nil {
		t.Fatal("Status is nil")
	}
	if ec.Status.Token != "token" {
		t.Errorf("Token = %q, want token", ec.Status.Token)
	}
	if ec.Status.ExpirationTimestamp == nil || !ec.Status.ExpirationTimestamp.Time.Equal(exp) {
		t.Errorf("ExpirationTimestamp = %v, want %v", ec.Status.ExpirationTimestamp, exp)
	}
}

func TestNewExecCredentialZeroExpiry(t *testing.T) {
	ec := newExecCredentialV1("token", time.Time{})
	if ec.Status.ExpirationTimestamp == nil {
		t.Fatal("ExpirationTimestamp is nil")
	}
	if !ec.Status.ExpirationTimestamp.IsZero() {
		t.Errorf("ExpirationTimestamp = %v, want zero", ec.Status.ExpirationTimestamp)
	}
}

func TestFormatJSON(t *testing.T) {
	t.Run("v1beta1 credential", func(t *testing.T) {
		exp := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
		want := `{
  "kind": "ExecCredential",
  "apiVersion": "client.authentication.k8s.io/v1beta1",
  "spec": {
    "interactive": false
  },
  "status": {
    "expirationTimestamp": "2030-01-02T03:04:05Z",
    "token": "abc"
  }
}`
		if got := formatJSON(newExecCredentialV1Beta1("abc.", exp)); got != want {
			t.Errorf("formatJSON() =\n%s\nwant\n%s", got, want)
		}
	})

	t.Run("v1 credential is valid parseable json", func(t *testing.T) {
		exp := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
		out := formatJSON(newExecCredentialV1("abc", exp))
		var parsed clientauthv1.ExecCredential
		if err := json.Unmarshal([]byte(out), &parsed); err != nil {
			t.Fatalf("output is not valid json: %v\n%s", err, out)
		}
		if parsed.APIVersion != "client.authentication.k8s.io/v1" || parsed.Kind != "ExecCredential" {
			t.Errorf("TypeMeta = %+v", parsed.TypeMeta)
		}
		if parsed.Status == nil || parsed.Status.Token != "abc" {
			t.Errorf("Status = %+v", parsed.Status)
		}
		if parsed.Status.ExpirationTimestamp == nil || !parsed.Status.ExpirationTimestamp.Time.Equal(exp) {
			t.Errorf("ExpirationTimestamp = %v, want %v", parsed.Status.ExpirationTimestamp, exp)
		}
	})

	t.Run("uses two space indentation", func(t *testing.T) {
		out := formatJSON(map[string]map[string]int{"a": {"b": 1}})
		want := "{\n  \"a\": {\n    \"b\": 1\n  }\n}"
		if out != want {
			t.Errorf("formatJSON() = %q, want %q", out, want)
		}
	})

	t.Run("nil", func(t *testing.T) {
		if got := formatJSON(nil); got != "null" {
			t.Errorf("formatJSON(nil) = %q, want null", got)
		}
	})

	t.Run("unmarshalable value yields empty string", func(t *testing.T) {
		if got := formatJSON(make(chan int)); got != "" {
			t.Errorf("formatJSON(chan) = %q, want empty", got)
		}
	})
}

func TestGcpUnknownClientAuthVersion(t *testing.T) {
	isolateGcp(t)
	setupCacheDir(t)
	for _, version := range []string{"", "v2", "V1", "v1alpha1", "bogus"} {
		t.Run("version="+version, func(t *testing.T) {
			var err error
			out := captureStdout(t, func() {
				err = Gcp(context.Background(), "", version)
			})
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			want := "unknown client auth version '" + version + "'"
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error = %q, want it to contain %q", err.Error(), want)
			}
			if out != "" {
				t.Errorf("unexpected output %q", out)
			}
		})
	}
}

func TestGcpUsesCachedCredential(t *testing.T) {
	exp := time.Now().Add(time.Hour).UTC().Truncate(time.Second)

	t.Run("v1beta1", func(t *testing.T) {
		isolateGcp(t)
		cl := setupCacheDir(t)
		cached := newExecCredentialV1Beta1("cached-beta", exp)
		if err := saveFile(cl, cached); err != nil {
			t.Fatal(err)
		}
		var err error
		out := captureStdout(t, func() {
			err = Gcp(context.Background(), "", "v1beta1")
		})
		if err != nil {
			t.Fatalf("Gcp() error = %v", err)
		}
		if want := formatJSON(cached); out != want {
			t.Errorf("output =\n%s\nwant\n%s", out, want)
		}
	})

	t.Run("v1", func(t *testing.T) {
		isolateGcp(t)
		cl := setupCacheDir(t)
		cached := newExecCredentialV1("cached-v1", exp)
		if err := saveFile(cl, cached); err != nil {
			t.Fatal(err)
		}
		var err error
		out := captureStdout(t, func() {
			err = Gcp(context.Background(), "", "v1")
		})
		if err != nil {
			t.Fatalf("Gcp() error = %v", err)
		}
		if want := formatJSON(cached); out != want {
			t.Errorf("output =\n%s\nwant\n%s", out, want)
		}
	})

	t.Run("cached credential is used even when impersonating", func(t *testing.T) {
		isolateGcp(t)
		cl := setupCacheDir(t)
		cached := newExecCredentialV1("cached-v1", exp)
		if err := saveFile(cl, cached); err != nil {
			t.Fatal(err)
		}
		var err error
		out := captureStdout(t, func() {
			err = Gcp(context.Background(), "sa@project.iam.gserviceaccount.com", "v1")
		})
		if err != nil {
			t.Fatalf("Gcp() error = %v", err)
		}
		if want := formatJSON(cached); out != want {
			t.Errorf("output =\n%s\nwant\n%s", out, want)
		}
	})
}

func TestGcpDefaultCredentialsFailure(t *testing.T) {
	for _, version := range []string{"v1beta1", "v1"} {
		t.Run("missing credentials file/"+version, func(t *testing.T) {
			isolateGcp(t)
			cl := setupCacheDir(t)
			var err error
			out := captureStdout(t, func() {
				err = Gcp(context.Background(), "", version)
			})
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), "GOOGLE_APPLICATION_CREDENTIALS") {
				t.Errorf("error = %q, want it to mention GOOGLE_APPLICATION_CREDENTIALS", err.Error())
			}
			if out != "" {
				t.Errorf("unexpected output %q", out)
			}
			if fileExists(t, cl) {
				t.Error("cache file written on failure")
			}
		})

		t.Run("expired cache falls through to credentials/"+version, func(t *testing.T) {
			isolateGcp(t)
			cl := setupCacheDir(t)
			if err := saveFile(cl, newExecCredentialV1("old", time.Now().Add(-time.Hour))); err != nil {
				t.Fatal(err)
			}
			var err error
			_ = captureStdout(t, func() {
				err = Gcp(context.Background(), "", version)
			})
			if err == nil {
				t.Fatal("expected error from default credentials, got nil")
			}
			if fileExists(t, cl) {
				t.Error("expired cache file was not deleted")
			}
		})
	}

	t.Run("malformed credentials file", func(t *testing.T) {
		isolateGcp(t)
		setupCacheDir(t)
		credFile := filepath.Join(t.TempDir(), "creds.json")
		if err := os.WriteFile(credFile, []byte("{not json"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", credFile)
		err := Gcp(context.Background(), "", "v1")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
