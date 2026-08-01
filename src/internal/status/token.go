package status

import (
	"fmt"
	"net/http"
	"os"
	"strings"
)

// get is every request a provider makes: the client Fetch chose, plus the
// credential when the operator named a file holding one.
//
// One helper rather than a header set in three places, because the rule that
// matters (the token is never written in site.yaml and never printed) is
// easier to keep in one function than to remember in each provider.
func get(client *http.Client, src Source, addr string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, addr, nil)
	if err != nil {
		return nil, err
	}
	if src.TokenFile != "" {
		token, err := os.ReadFile(src.TokenFile)
		if err != nil {
			return nil, fmt.Errorf("status.token_file %s could not be read: %w (it is a path the platform mounts, as a kubernetes secret, a docker secret or a vault agent file does)", src.TokenFile, err)
		}
		// Trimmed because every editor and every kubectl create secret
		// --from-file leaves a trailing newline, and a header carrying one is
		// a header the far end rejects for no reason anybody can see.
		scheme := src.TokenScheme
		if scheme == "" {
			scheme = "Bearer"
		}
		req.Header.Set("Authorization", scheme+" "+strings.TrimSpace(string(token)))
	}
	return client.Do(req)
}

// answered turns a status code into a message an operator can act on. 401 and
// 403 are the two that name a key rather than an address: the credential is
// the thing that was refused, and it is the one part of the request that never
// appears in a log.
func answered(resp *http.Response, src Source) error {
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		if src.TokenFile == "" {
			return fmt.Errorf("status.url %s answered %s, and cairn sent no credential (set status.token_file to a file holding the token)", src.URL, resp.Status)
		}
		return fmt.Errorf("status.url %s answered %s: the token in status.token_file %s was refused (check the file holds the token alone, and that status.token_scheme matches what the API expects)", src.URL, resp.Status, src.TokenFile)
	default:
		return fmt.Errorf("status.url %s answered %s", src.URL, resp.Status)
	}
}
