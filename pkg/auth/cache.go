package auth

import (
	"bufio"
	"github.com/traviswt/gke-auth-plugin/pkg/conf"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/pkg/apis/clientauthentication/v1beta1"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

func GetExecCredential() *v1beta1.ExecCredential {
	cl := cacheLocation()
	if cl == "" {
		return nil
	}
	ec, err := loadFile(cl)
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

func SaveExecCredential(ec *v1beta1.ExecCredential) {
	doNotCache := os.Getenv("GKE_AUTH_PLUGIN_DO_NOT_CACHE")
	if doNotCache != "" && strings.ToLower(doNotCache) == "true" {
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

func loadFile(file string) (*v1beta1.ExecCredential, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var ec v1beta1.ExecCredential
	err = yaml.Unmarshal(data, &ec)
	if err != nil {
		return nil, err
	}
	return &ec, nil
}

func saveFile(file string, ec *v1beta1.ExecCredential) error {
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
