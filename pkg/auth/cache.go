package auth

import (
	"bufio"
	"github.com/traviswt/gke-auth-plugin/pkg/conf"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/client-go/pkg/apis/clientauthentication/v1"
	"k8s.io/client-go/pkg/apis/clientauthentication/v1beta1"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type ExecCredentialTypes interface {
	v1beta1.ExecCredential |
		v1.ExecCredential
}

func GetExecCredentialV1Beta1() *v1beta1.ExecCredential {
	cl := cacheLocation()
	if cl == "" {
		return nil
	}
	ec, err := loadFile[v1beta1.ExecCredential](cl)
	if err != nil {
		return nil
	}
	if ec != nil {
		if ec.Status != nil && ec.Status.ExpirationTimestamp != nil {
			now := metav1.NewTime(time.Now())
			if ec.Status.ExpirationTimestamp.Before(&now) {
				deleteFile(cl)
				return nil
			}
		}
	}
	return ec
}

func GetExecCredentialV1() *v1.ExecCredential {
	cl := cacheLocation()
	if cl == "" {
		return nil
	}
	ec, err := loadFile[v1.ExecCredential](cl)
	if err != nil {
		return nil
	}
	if ec != nil {
		if ec.Status != nil && ec.Status.ExpirationTimestamp != nil {
			now := metav1.NewTime(time.Now())
			if ec.Status.ExpirationTimestamp.Before(&now) {
				deleteFile(cl)
				return nil
			}
		}
	}
	return ec
}

func SaveExecCredential[T ExecCredentialTypes](ec *T) {
	doNotCache := os.Getenv("GKE_AUTH_PLUGIN_DO_NOT_CACHE")
	if strings.ToLower(doNotCache) == "true" {
		return
	}
	cl := cacheLocation()
	if cl == "" {
		return
	}
	_ = saveFile(cl, ec)
}

// cacheLocation returns the file to Cache the exec cred to, if blank, don't Cache
func cacheLocation() string {
	cacheFileDir := ""
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig != "" {
		abs, err := filepath.Abs(kubeconfig)
		if err == nil {
			dir := filepath.Dir(abs)
			cacheFileDir = dir
		}
	}
	if cacheFileDir == "" {
		return ""
	}
	cf := path.Join(cacheFileDir, conf.CacheFileName)
	return cf
}

func loadFile[T ExecCredentialTypes](file string) (*T, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var ec T
	err = yaml.Unmarshal(data, &ec)
	if err != nil {
		return nil, err
	}
	return &ec, nil
}

func saveFile[T ExecCredentialTypes](file string, ec *T) error {
	if ec == nil {
		return nil
	}
	data, err := yaml.Marshal(ec)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	deleteFile(file)
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	_, err = w.Write(data)
	if err != nil {
		return err
	}
	if err := w.Flush(); err != nil {
		return err
	}
	return nil
}

func deleteFile(file string) {
	_ = os.Remove(file)
}
