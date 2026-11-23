package keycloak

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// ErrServiceAccountNotFound means the client does not exist (exact client ID)
// or has no service account.
var ErrServiceAccountNotFound = errors.New("keycloak client or its service account not found")

// ServiceAccountUserID returns the Keycloak user ID of a client's service
// account: the subject of the client's client-credentials tokens.
func (c *KeyAdminClient) ServiceAccountUserID(ctx context.Context, clientID string) (string, error) {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return "", ErrServiceAccountNotFound
	}
	query := url.Values{"clientId": {clientID}}
	res, err := c.GetWithContext(ctx, "clients?"+query.Encode())
	if err != nil {
		return "", err
	}
	var clients []struct {
		ID                     string `json:"id"`
		ClientID               string `json:"clientId"`
		ServiceAccountsEnabled bool   `json:"serviceAccountsEnabled"`
	}
	err = decodeOK(res, &clients, "list clients")
	if err != nil {
		return "", err
	}
	uuid := ""
	for _, client := range clients {
		if client.ClientID == clientID && client.ServiceAccountsEnabled {
			uuid = client.ID
		}
	}
	if uuid == "" {
		return "", ErrServiceAccountNotFound
	}
	res, err = c.GetWithContext(ctx, "clients/"+url.PathEscape(uuid)+"/service-account-user")
	if err != nil {
		return "", err
	}
	if res.StatusCode == http.StatusNotFound {
		res.Body.Close()
		return "", ErrServiceAccountNotFound
	}
	var user struct {
		ID string `json:"id"`
	}
	if err := decodeOK(res, &user, "get service account user"); err != nil {
		return "", err
	}
	if user.ID == "" {
		return "", ErrServiceAccountNotFound
	}
	return user.ID, nil
}

func decodeOK(res *http.Response, out any, what string) error {
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return fmt.Errorf("%s failed [%d]: %s", what, res.StatusCode, string(body))
	}
	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", what, err)
	}
	return nil
}
