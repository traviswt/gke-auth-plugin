package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/impersonate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientauthv1beta1 "k8s.io/client-go/pkg/apis/clientauthentication/v1beta1"
	"strings"
	"time"
)

var (
	gcpScopes = []string{
		"https://www.googleapis.com/auth/cloud-platform",
		"https://www.googleapis.com/auth/userinfo.email",
	}
)

func Gcp(ctx context.Context, impersonationAccount string) error {
	// Use cached exec credential
	if ec := GetExecCredential(); ec != nil {
		credString := formatJSON(ec)
		fmt.Print(credString)
		return nil
	}

	var ts oauth2.TokenSource
	var err error
	if impersonationAccount != "" {
		// Get impersonated token source
		ts, err = impersonate.CredentialsTokenSource(ctx, impersonate.CredentialsConfig{
			TargetPrincipal: impersonationAccount,
			Scopes:          gcpScopes,
		})
	} else {
		// Get application default credentials token source
		cred, err := google.FindDefaultCredentials(ctx, gcpScopes...)
		if err != nil {
			return err
		}
		if cred == nil {
			return errors.New("failed finding default credentials")
		}
		ts = cred.TokenSource
	}
	if err != nil {
		return err
	}

	token, err := ts.Token()
	if err != nil {
		return err
	}

	// Create ExecCredential from token
	ec := newExecCredential(token.AccessToken, token.Expiry)

	// Cache exec credential
	SaveExecCredential(ec)
	credString := formatJSON(ec)
	fmt.Print(credString)
	return nil
}

func formatJSON(ec *clientauthv1beta1.ExecCredential) string {
	//pretty print
	enc, _ := json.MarshalIndent(ec, "", "  ")
	return string(enc)
}

func newExecCredential(token string, exp time.Time) *clientauthv1beta1.ExecCredential {
	metaExp := metav1.NewTime(exp)
	//the google token sometimes contains trailing periods,
	//they cause problems with various tools, thus right trim
	token = strings.TrimRightFunc(token, func(r rune) bool {
		if r == '.' {
			return true
		}
		return false
	})
	ec := &clientauthv1beta1.ExecCredential{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clientauthv1beta1.SchemeGroupVersion.Identifier(),
			Kind:       "ExecCredential",
		},
		Status: &clientauthv1beta1.ExecCredentialStatus{
			ExpirationTimestamp: &metaExp,
			Token:               token,
		},
	}
	return ec
}
