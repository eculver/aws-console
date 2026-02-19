package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	defaultFederationURL = "https://signin.aws.amazon.com/federation"
	defaultConsoleURL    = "https://console.aws.amazon.com/"
)

type federationHTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// FederationClient calls the AWS federation endpoint to build console URLs.
type FederationClient struct {
	client        federationHTTPClient
	federationURL string
	consoleURL    string
}

// NewFederationClient creates a federation client with sane defaults.
func NewFederationClient() *FederationClient {
	return newFederationClient(
		&http.Client{Timeout: 15 * time.Second},
		defaultFederationURL,
		defaultConsoleURL,
	)
}

func newFederationClient(client federationHTTPClient, federationURL string, consoleURL string) *FederationClient {
	return &FederationClient{
		client:        client,
		federationURL: federationURL,
		consoleURL:    consoleURL,
	}
}

func (f *FederationClient) BuildConsoleURL(ctx context.Context, creds Credentials, durationSeconds int32) (string, error) {
	sessionData := map[string]string{
		"sessionId":    creds.AccessKeyID,
		"sessionKey":   creds.SecretAccessKey,
		"sessionToken": creds.SessionToken,
	}

	sessionJSON, err := json.Marshal(sessionData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal session: %w", err)
	}

	tokenURL := fmt.Sprintf(
		"%s?Action=getSigninToken&SessionDuration=%d&Session=%s",
		f.federationURL,
		durationSeconds,
		url.QueryEscape(string(sessionJSON)),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to build federation request: %w", err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to request signin token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read federation response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("federation endpoint returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		SigninToken string `json:"SigninToken"`
	}

	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse signin token response: %w", err)
	}

	if tokenResp.SigninToken == "" {
		return "", fmt.Errorf("received empty signin token from federation endpoint")
	}

	loginURL := fmt.Sprintf(
		"%s?Action=login&Issuer=aws-console-cli&Destination=%s&SigninToken=%s",
		f.federationURL,
		url.QueryEscape(f.consoleURL),
		url.QueryEscape(tokenResp.SigninToken),
	)

	return loginURL, nil
}
