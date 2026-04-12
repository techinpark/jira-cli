package auth

import (
	"encoding/base64"
	"testing"

	"github.com/techinpark/jira-cli/internal/config"
)

func TestBasicAuthHeader(t *testing.T) {
	header := BasicAuthHeader(config.ResolvedProfile{
		Email:    "user@example.com",
		APIToken: "secret",
	})
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("user@example.com:secret"))
	if header != want {
		t.Fatalf("unexpected header: %s", header)
	}
}
