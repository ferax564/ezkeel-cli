package forgejo

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// CreateRunnerRegistrationToken requests a new runner registration token
// from the Forgejo admin API. Requires admin-level API token.
func (c *Client) CreateRunnerRegistrationToken() (string, error) {
	data, err := c.do(http.MethodGet, "/api/v1/user/actions/runners/registration-token", nil)
	if err != nil {
		return "", fmt.Errorf("requesting runner token: %w", err)
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("parsing runner token response: %w", err)
	}

	if result.Token == "" {
		return "", fmt.Errorf("API returned empty runner registration token")
	}

	return result.Token, nil
}
