package aws

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type fakeHTTPClient struct {
	doFunc func(req *http.Request) (*http.Response, error)
}

func (f fakeHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return f.doFunc(req)
}

func TestFederationClientBuildConsoleURL(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		responseBody  string
		statusCode    int
		wantErrSubstr string
		assertSuccess func(t *testing.T, loginURL string)
	}{
		{
			name:         "success",
			responseBody: `{"SigninToken":"token-123"}`,
			statusCode:   http.StatusOK,
			assertSuccess: func(t *testing.T, loginURL string) {
				t.Helper()
				parsed, err := url.Parse(loginURL)
				if err != nil {
					t.Fatalf("failed to parse login URL: %v", err)
				}
				if parsed.Query().Get("Action") != "login" {
					t.Fatalf("unexpected login action: %q", parsed.Query().Get("Action"))
				}
				if parsed.Query().Get("SigninToken") != "token-123" {
					t.Fatalf("unexpected sign-in token: %q", parsed.Query().Get("SigninToken"))
				}
			},
		},
		{
			name:          "non-200 response",
			responseBody:  "forbidden",
			statusCode:    http.StatusForbidden,
			wantErrSubstr: "HTTP 403",
		},
		{
			name:          "invalid json response",
			responseBody:  "{not-json}",
			statusCode:    http.StatusOK,
			wantErrSubstr: "failed to parse signin token response",
		},
		{
			name:          "empty signin token",
			responseBody:  `{"SigninToken":""}`,
			statusCode:    http.StatusOK,
			wantErrSubstr: "received empty signin token from federation endpoint",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("Action") != "getSigninToken" {
					t.Fatalf("unexpected action: %q", r.URL.Query().Get("Action"))
				}
				w.WriteHeader(tc.statusCode)
				_, _ = w.Write([]byte(tc.responseBody))
			}))
			defer server.Close()

			client := newFederationClient(server.Client(), server.URL, "https://console.aws.amazon.com/")
			loginURL, err := client.BuildConsoleURL(context.Background(), Credentials{
				AccessKeyID:     "AKIA_TEST",
				SecretAccessKey: "secret",
				SessionToken:    "token",
			}, 3600)

			if tc.wantErrSubstr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q but got nil", tc.wantErrSubstr)
				}
				if !strings.Contains(err.Error(), tc.wantErrSubstr) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErrSubstr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("BuildConsoleURL returned error: %v", err)
			}

			if tc.assertSuccess != nil {
				tc.assertSuccess(t, loginURL)
			}
		})
	}
}

func TestFederationClientBuildConsoleURLClientError(t *testing.T) {
	t.Parallel()

	client := newFederationClient(fakeHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("network error")
		},
	}, "https://signin.aws.amazon.com/federation", "https://console.aws.amazon.com/")

	_, err := client.BuildConsoleURL(context.Background(), Credentials{
		AccessKeyID:     "AKIA_TEST",
		SecretAccessKey: "secret",
		SessionToken:    "token",
	}, 3600)
	if err == nil {
		t.Fatal("expected error but got nil")
	}
	if !strings.Contains(err.Error(), "failed to request signin token: network error") {
		t.Fatalf("unexpected error: %v", err)
	}
}
