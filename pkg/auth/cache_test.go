package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/traviswt/gke-auth-plugin/pkg/conf"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/client-go/pkg/apis/clientauthentication/v1"
	"k8s.io/client-go/pkg/apis/clientauthentication/v1beta1"
)

// setupCacheDir points KUBECONFIG at a file inside a fresh temp dir and
// returns the expected cache file location.
func setupCacheDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("KUBECONFIG", filepath.Join(dir, "config"))
	t.Setenv("GKE_AUTH_PLUGIN_DO_NOT_CACHE", "")
	return filepath.Join(dir, conf.CacheFileName)
}

func fileExists(t *testing.T, file string) bool {
	t.Helper()
	_, err := os.Stat(file)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	t.Fatalf("unexpected stat error for %s: %v", file, err)
	return false
}

func TestCacheLocation(t *testing.T) {
	t.Run("empty when KUBECONFIG unset", func(t *testing.T) {
		t.Setenv("KUBECONFIG", "")
		if got := cacheLocation(); got != "" {
			t.Errorf("cacheLocation() = %q, want empty", got)
		}
	})

	t.Run("uses directory of absolute KUBECONFIG", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("KUBECONFIG", filepath.Join(dir, "kubeconfig.yaml"))
		want := filepath.Join(dir, conf.CacheFileName)
		if got := cacheLocation(); got != want {
			t.Errorf("cacheLocation() = %q, want %q", got, want)
		}
	})

	t.Run("resolves relative KUBECONFIG against working directory", func(t *testing.T) {
		wd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		t.Setenv("KUBECONFIG", filepath.Join("some", "nested", "config"))
		want := filepath.Join(wd, "some", "nested", conf.CacheFileName)
		if got := cacheLocation(); got != want {
			t.Errorf("cacheLocation() = %q, want %q", got, want)
		}
	})

	t.Run("does not require the KUBECONFIG file to exist", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("KUBECONFIG", filepath.Join(dir, "missing"))
		want := filepath.Join(dir, conf.CacheFileName)
		if got := cacheLocation(); got != want {
			t.Errorf("cacheLocation() = %q, want %q", got, want)
		}
	})
}

func TestSaveAndLoadFileRoundTrip(t *testing.T) {
	exp := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)

	t.Run("v1beta1", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "cache")
		in := newExecCredentialV1Beta1("tok-beta", exp)
		if err := saveFile(file, in); err != nil {
			t.Fatalf("saveFile() error = %v", err)
		}
		out, err := loadFile[v1beta1.ExecCredential](file)
		if err != nil {
			t.Fatalf("loadFile() error = %v", err)
		}
		assertV1Beta1Equal(t, out, in)
	})

	t.Run("v1", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "cache")
		in := newExecCredentialV1("tok-v1", exp)
		if err := saveFile(file, in); err != nil {
			t.Fatalf("saveFile() error = %v", err)
		}
		out, err := loadFile[v1.ExecCredential](file)
		if err != nil {
			t.Fatalf("loadFile() error = %v", err)
		}
		assertV1Equal(t, out, in)
	})
}

func TestSaveFile(t *testing.T) {
	t.Run("nil credential is a no-op", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "cache")
		if err := saveFile[v1.ExecCredential](file, nil); err != nil {
			t.Fatalf("saveFile(nil) error = %v", err)
		}
		if fileExists(t, file) {
			t.Error("saveFile(nil) created a file")
		}
	})

	t.Run("overwrites existing file", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "cache")
		exp := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
		if err := saveFile(file, newExecCredentialV1("first", exp)); err != nil {
			t.Fatal(err)
		}
		if err := saveFile(file, newExecCredentialV1("second", exp)); err != nil {
			t.Fatal(err)
		}
		out, err := loadFile[v1.ExecCredential](file)
		if err != nil {
			t.Fatal(err)
		}
		if out.Status == nil || out.Status.Token != "second" {
			t.Errorf("token after overwrite = %+v, want %q", out.Status, "second")
		}
	})

	t.Run("returns error when directory does not exist", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "does", "not", "exist", "cache")
		err := saveFile(file, newExecCredentialV1("tok", time.Now()))
		if err == nil {
			t.Fatal("saveFile() expected error for missing directory, got nil")
		}
	})
}

func TestLoadFile(t *testing.T) {
	t.Run("missing file returns error", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "missing")
		ec, err := loadFile[v1.ExecCredential](file)
		if err == nil {
			t.Fatal("loadFile() expected error, got nil")
		}
		if ec != nil {
			t.Errorf("loadFile() = %+v, want nil", ec)
		}
	})

	t.Run("invalid yaml returns error", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "bad")
		if err := os.WriteFile(file, []byte("status: [unclosed"), 0o600); err != nil {
			t.Fatal(err)
		}
		ec, err := loadFile[v1beta1.ExecCredential](file)
		if err == nil {
			t.Fatal("loadFile() expected error, got nil")
		}
		if ec != nil {
			t.Errorf("loadFile() = %+v, want nil", ec)
		}
	})

	t.Run("empty file yields zero value credential", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "empty")
		if err := os.WriteFile(file, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		ec, err := loadFile[v1.ExecCredential](file)
		if err != nil {
			t.Fatalf("loadFile() error = %v", err)
		}
		if ec == nil {
			t.Fatal("loadFile() = nil, want zero value")
		}
		if ec.Status != nil {
			t.Errorf("Status = %+v, want nil", ec.Status)
		}
	})
}

func TestDeleteFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "cache")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	deleteFile(file)
	if fileExists(t, file) {
		t.Error("deleteFile() did not remove file")
	}
	// deleting a missing file must not panic
	deleteFile(file)
}

func TestSaveExecCredential(t *testing.T) {
	exp := time.Now().Add(time.Hour).UTC().Truncate(time.Second)

	t.Run("writes v1beta1 cache file", func(t *testing.T) {
		cl := setupCacheDir(t)
		SaveExecCredential(newExecCredentialV1Beta1("tok", exp))
		out, err := loadFile[v1beta1.ExecCredential](cl)
		if err != nil {
			t.Fatalf("cache not written: %v", err)
		}
		if out.Status == nil || out.Status.Token != "tok" {
			t.Errorf("cached status = %+v, want token %q", out.Status, "tok")
		}
	})

	t.Run("writes v1 cache file", func(t *testing.T) {
		cl := setupCacheDir(t)
		SaveExecCredential(newExecCredentialV1("tok", exp))
		out, err := loadFile[v1.ExecCredential](cl)
		if err != nil {
			t.Fatalf("cache not written: %v", err)
		}
		if out.Status == nil || out.Status.Token != "tok" {
			t.Errorf("cached status = %+v, want token %q", out.Status, "tok")
		}
	})

	for _, val := range []string{"true", "TRUE", "True"} {
		t.Run("skipped when GKE_AUTH_PLUGIN_DO_NOT_CACHE="+val, func(t *testing.T) {
			cl := setupCacheDir(t)
			t.Setenv("GKE_AUTH_PLUGIN_DO_NOT_CACHE", val)
			SaveExecCredential(newExecCredentialV1("tok", exp))
			if fileExists(t, cl) {
				t.Error("cache file written despite GKE_AUTH_PLUGIN_DO_NOT_CACHE")
			}
		})
	}

	for _, val := range []string{"false", "1", "yes"} {
		t.Run("still caches when GKE_AUTH_PLUGIN_DO_NOT_CACHE="+val, func(t *testing.T) {
			cl := setupCacheDir(t)
			t.Setenv("GKE_AUTH_PLUGIN_DO_NOT_CACHE", val)
			SaveExecCredential(newExecCredentialV1("tok", exp))
			if !fileExists(t, cl) {
				t.Error("cache file not written")
			}
		})
	}

	t.Run("skipped when KUBECONFIG unset", func(t *testing.T) {
		wd := t.TempDir()
		t.Chdir(wd)
		t.Setenv("KUBECONFIG", "")
		t.Setenv("GKE_AUTH_PLUGIN_DO_NOT_CACHE", "")
		SaveExecCredential(newExecCredentialV1("tok", exp))
		if fileExists(t, filepath.Join(wd, conf.CacheFileName)) {
			t.Error("cache file written without KUBECONFIG")
		}
	})

	t.Run("nil credential does not create file", func(t *testing.T) {
		cl := setupCacheDir(t)
		SaveExecCredential[v1.ExecCredential](nil)
		if fileExists(t, cl) {
			t.Error("cache file written for nil credential")
		}
	})
}

func TestGetExecCredentialV1Beta1(t *testing.T) {
	t.Run("nil when KUBECONFIG unset", func(t *testing.T) {
		t.Setenv("KUBECONFIG", "")
		if ec := GetExecCredentialV1Beta1(); ec != nil {
			t.Errorf("got %+v, want nil", ec)
		}
	})

	t.Run("nil when cache file missing", func(t *testing.T) {
		setupCacheDir(t)
		if ec := GetExecCredentialV1Beta1(); ec != nil {
			t.Errorf("got %+v, want nil", ec)
		}
	})

	t.Run("nil when cache file is corrupt", func(t *testing.T) {
		cl := setupCacheDir(t)
		if err := os.WriteFile(cl, []byte("status: [unclosed"), 0o600); err != nil {
			t.Fatal(err)
		}
		if ec := GetExecCredentialV1Beta1(); ec != nil {
			t.Errorf("got %+v, want nil", ec)
		}
	})

	t.Run("returns valid cached credential", func(t *testing.T) {
		cl := setupCacheDir(t)
		in := newExecCredentialV1Beta1("cached", time.Now().Add(time.Hour).UTC().Truncate(time.Second))
		if err := saveFile(cl, in); err != nil {
			t.Fatal(err)
		}
		assertV1Beta1Equal(t, GetExecCredentialV1Beta1(), in)
		if !fileExists(t, cl) {
			t.Error("valid cache file was removed")
		}
	})

	t.Run("expired credential returns nil and deletes cache", func(t *testing.T) {
		cl := setupCacheDir(t)
		in := newExecCredentialV1Beta1("old", time.Now().Add(-time.Minute))
		if err := saveFile(cl, in); err != nil {
			t.Fatal(err)
		}
		if ec := GetExecCredentialV1Beta1(); ec != nil {
			t.Errorf("got %+v, want nil", ec)
		}
		if fileExists(t, cl) {
			t.Error("expired cache file was not deleted")
		}
	})

	t.Run("credential without status is returned", func(t *testing.T) {
		cl := setupCacheDir(t)
		if err := saveFile(cl, &v1beta1.ExecCredential{}); err != nil {
			t.Fatal(err)
		}
		ec := GetExecCredentialV1Beta1()
		if ec == nil {
			t.Fatal("got nil, want credential")
		}
		if ec.Status != nil {
			t.Errorf("Status = %+v, want nil", ec.Status)
		}
	})

	t.Run("credential without expiration is returned", func(t *testing.T) {
		cl := setupCacheDir(t)
		in := &v1beta1.ExecCredential{Status: &v1beta1.ExecCredentialStatus{Token: "forever"}}
		if err := saveFile(cl, in); err != nil {
			t.Fatal(err)
		}
		ec := GetExecCredentialV1Beta1()
		if ec == nil || ec.Status == nil || ec.Status.Token != "forever" {
			t.Errorf("got %+v, want token %q", ec, "forever")
		}
	})
}

func TestGetExecCredentialV1(t *testing.T) {
	t.Run("nil when KUBECONFIG unset", func(t *testing.T) {
		t.Setenv("KUBECONFIG", "")
		if ec := GetExecCredentialV1(); ec != nil {
			t.Errorf("got %+v, want nil", ec)
		}
	})

	t.Run("nil when cache file missing", func(t *testing.T) {
		setupCacheDir(t)
		if ec := GetExecCredentialV1(); ec != nil {
			t.Errorf("got %+v, want nil", ec)
		}
	})

	t.Run("nil when cache file is corrupt", func(t *testing.T) {
		cl := setupCacheDir(t)
		if err := os.WriteFile(cl, []byte("status: [unclosed"), 0o600); err != nil {
			t.Fatal(err)
		}
		if ec := GetExecCredentialV1(); ec != nil {
			t.Errorf("got %+v, want nil", ec)
		}
	})

	t.Run("returns valid cached credential", func(t *testing.T) {
		cl := setupCacheDir(t)
		in := newExecCredentialV1("cached", time.Now().Add(time.Hour).UTC().Truncate(time.Second))
		if err := saveFile(cl, in); err != nil {
			t.Fatal(err)
		}
		assertV1Equal(t, GetExecCredentialV1(), in)
		if !fileExists(t, cl) {
			t.Error("valid cache file was removed")
		}
	})

	t.Run("expired credential returns nil and deletes cache", func(t *testing.T) {
		cl := setupCacheDir(t)
		in := newExecCredentialV1("old", time.Now().Add(-time.Minute))
		if err := saveFile(cl, in); err != nil {
			t.Fatal(err)
		}
		if ec := GetExecCredentialV1(); ec != nil {
			t.Errorf("got %+v, want nil", ec)
		}
		if fileExists(t, cl) {
			t.Error("expired cache file was not deleted")
		}
	})

	t.Run("credential without status is returned", func(t *testing.T) {
		cl := setupCacheDir(t)
		if err := saveFile(cl, &v1.ExecCredential{}); err != nil {
			t.Fatal(err)
		}
		ec := GetExecCredentialV1()
		if ec == nil {
			t.Fatal("got nil, want credential")
		}
		if ec.Status != nil {
			t.Errorf("Status = %+v, want nil", ec.Status)
		}
	})

	t.Run("credential without expiration is returned", func(t *testing.T) {
		cl := setupCacheDir(t)
		in := &v1.ExecCredential{Status: &v1.ExecCredentialStatus{Token: "forever"}}
		if err := saveFile(cl, in); err != nil {
			t.Fatal(err)
		}
		ec := GetExecCredentialV1()
		if ec == nil || ec.Status == nil || ec.Status.Token != "forever" {
			t.Errorf("got %+v, want token %q", ec, "forever")
		}
	})
}

func assertV1Beta1Equal(t *testing.T, got, want *v1beta1.ExecCredential) {
	t.Helper()
	if got == nil {
		t.Fatal("credential is nil")
	}
	if got.APIVersion != want.APIVersion || got.Kind != want.Kind {
		t.Errorf("TypeMeta = %+v, want %+v", got.TypeMeta, want.TypeMeta)
	}
	if got.Status == nil {
		t.Fatal("Status is nil")
	}
	if got.Status.Token != want.Status.Token {
		t.Errorf("Token = %q, want %q", got.Status.Token, want.Status.Token)
	}
	assertTimeEqual(t, got.Status.ExpirationTimestamp, want.Status.ExpirationTimestamp)
}

func assertV1Equal(t *testing.T, got, want *v1.ExecCredential) {
	t.Helper()
	if got == nil {
		t.Fatal("credential is nil")
	}
	if got.APIVersion != want.APIVersion || got.Kind != want.Kind {
		t.Errorf("TypeMeta = %+v, want %+v", got.TypeMeta, want.TypeMeta)
	}
	if got.Status == nil {
		t.Fatal("Status is nil")
	}
	if got.Status.Token != want.Status.Token {
		t.Errorf("Token = %q, want %q", got.Status.Token, want.Status.Token)
	}
	assertTimeEqual(t, got.Status.ExpirationTimestamp, want.Status.ExpirationTimestamp)
}

func assertTimeEqual(t *testing.T, got, want *metav1.Time) {
	t.Helper()
	if got == nil || want == nil {
		if got != want {
			t.Errorf("ExpirationTimestamp = %v, want %v", got, want)
		}
		return
	}
	if !got.Time.Equal(want.Time) {
		t.Errorf("ExpirationTimestamp = %v, want %v", got.Time, want.Time)
	}
}
