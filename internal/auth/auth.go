package auth

import (
	"encoding/base64"

	"github.com/techinpark/jira-cli/internal/config"
)

func BasicAuthHeader(profile config.ResolvedProfile) string {
	token := base64.StdEncoding.EncodeToString([]byte(profile.Email + ":" + profile.APIToken))
	return "Basic " + token
}
