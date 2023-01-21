package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/oauth2/google"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientauthv1beta1 "k8s.io/client-go/pkg/apis/clientauthentication/v1beta1"
	"os"
	"strings"
	"time"
)

var (
	gcpScopes = []string{
		"https://www.googleapis.com/auth/cloud-platform",
		"https://www.googleapis.com/auth/userinfo.email",
	}
)

func Gcp(ctx context.Context) error {
	cred, err := google.FindDefaultCredentials(ctx, gcpScopes...)
	if err != nil {
		return err
	}
	if cred == nil {
		return errors.New("failed finding default credentials, cred is nil")
	}
	token, err := cred.TokenSource.Token()
	if err != nil {
		return err
	}
	if token == nil {
		return errors.New("failed retrieving token from credentials")
	}
	_, _ = fmt.Fprint(os.Stdout, formatJSON(token.AccessToken, token.Expiry))
	return nil
}

func formatJSON(token string, exp time.Time) string {
	metaExp := metav1.NewTime(exp)
	//the google token sometimes contains trailing periods,
	//they cause problems with various tools, thus right trim
	token = strings.TrimRightFunc(token, func(r rune) bool {
		if r == '.' {
			return true
		}
		return false
	})
	clientauthv1beta1.SchemeGroupVersion.Identifier()
	execInput := &clientauthv1beta1.ExecCredential{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clientauthv1beta1.SchemeGroupVersion.Identifier(),
			Kind:       "ExecCredential",
		},
		Status: &clientauthv1beta1.ExecCredentialStatus{
			ExpirationTimestamp: &metaExp,
			Token:               token,
		},
	}
	//pretty print
	enc, _ := json.MarshalIndent(execInput, "", "  ")
	return string(enc)
}
