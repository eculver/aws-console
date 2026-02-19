package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	awslib "github.com/eculver/aws-console/pkg/aws"
	"github.com/eculver/aws-console/pkg/aws/mocks"
)

type workflowState struct {
	stdout               bytes.Buffer
	stderr               bytes.Buffer
	openedURL            string
	openErr              error
	loginErr             error
	loginCalls           int
	lastLoginProfile     string
	expectedLoginProfile string
}

type execCall struct {
	method string
	name   string
	args   []string
}

type fakeExecutor struct {
	runErr   error
	startErr error
	calls    []execCall
}

func (f *fakeExecutor) Run(name string, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	f.calls = append(f.calls, execCall{
		method: "run",
		name:   name,
		args:   append([]string(nil), args...),
	})
	return f.runErr
}

func (f *fakeExecutor) Start(name string, args []string) error {
	f.calls = append(f.calls, execCall{
		method: "start",
		name:   name,
		args:   append([]string(nil), args...),
	})
	return f.startErr
}

func TestRunWorkflow(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name       string
		profile    string
		setup      func(t *testing.T, svc *mocks.Service, federation *mocks.FederationBuilder, state *workflowState)
		wantErr    string
		assertions func(t *testing.T, svc *mocks.Service, federation *mocks.FederationBuilder, state *workflowState)
	}

	testCases := []testCase{
		{
			name:    "happy path with existing session token",
			profile: "dev-profile",
			setup: func(t *testing.T, svc *mocks.Service, federation *mocks.FederationBuilder, state *workflowState) {
				t.Helper()
				svc.GetCallerIdentityFunc = func(ctx context.Context, profile string) (awslib.Identity, error) {
					return awslib.Identity{Arn: "arn:aws:iam::123456789012:user/test"}, nil
				}
				svc.RetrieveCredentialsFunc = func(ctx context.Context, profile string) (awslib.Credentials, error) {
					return awslib.Credentials{
						AccessKeyID:     "AKIA_TEST",
						SecretAccessKey: "secret",
						SessionToken:    "token",
					}, nil
				}
				federation.BuildConsoleURLFunc = func(ctx context.Context, creds awslib.Credentials, durationSeconds int32) (string, error) {
					return "https://example.com/console-login", nil
				}
			},
			assertions: func(t *testing.T, svc *mocks.Service, federation *mocks.FederationBuilder, state *workflowState) {
				t.Helper()
				if state.openedURL != "https://example.com/console-login" {
					t.Fatalf("unexpected opened URL: %q", state.openedURL)
				}
				if svc.GetCallerIdentityCalls != 1 {
					t.Fatalf("expected 1 GetCallerIdentity call, got %d", svc.GetCallerIdentityCalls)
				}
				if svc.GetSessionTokenCalls != 0 {
					t.Fatalf("expected 0 GetSessionToken calls, got %d", svc.GetSessionTokenCalls)
				}
				if federation.BuildConsoleURLCalls != 1 {
					t.Fatalf("expected 1 BuildConsoleURL call, got %d", federation.BuildConsoleURLCalls)
				}
				if !strings.Contains(state.stdout.String(), "Authenticated as: arn:aws:iam::123456789012:user/test") {
					t.Fatalf("expected authenticated output, got: %q", state.stdout.String())
				}
			},
		},
		{
			name:    "falls back to SSO and requests session token",
			profile: "dev-profile",
			setup: func(t *testing.T, svc *mocks.Service, federation *mocks.FederationBuilder, state *workflowState) {
				t.Helper()
				state.expectedLoginProfile = "dev-profile"

				identityCalls := 0
				svc.GetCallerIdentityFunc = func(ctx context.Context, profile string) (awslib.Identity, error) {
					identityCalls++
					if identityCalls == 1 {
						return awslib.Identity{}, errors.New("expired token")
					}
					return awslib.Identity{Arn: "arn:aws:iam::123456789012:user/test"}, nil
				}
				svc.RetrieveCredentialsFunc = func(ctx context.Context, profile string) (awslib.Credentials, error) {
					return awslib.Credentials{
						AccessKeyID:     "AKIA_LONG",
						SecretAccessKey: "long-secret",
						SessionToken:    "",
					}, nil
				}
				svc.GetSessionTokenFunc = func(ctx context.Context, profile string, durationSeconds int32) (awslib.Credentials, error) {
					return awslib.Credentials{
						AccessKeyID:     "AKIA_TEMP",
						SecretAccessKey: "temp-secret",
						SessionToken:    "temp-token",
					}, nil
				}
				federation.BuildConsoleURLFunc = func(ctx context.Context, creds awslib.Credentials, durationSeconds int32) (string, error) {
					return "https://example.com/federated", nil
				}
			},
			assertions: func(t *testing.T, svc *mocks.Service, federation *mocks.FederationBuilder, state *workflowState) {
				t.Helper()
				if state.loginCalls != 1 {
					t.Fatalf("expected login to be called once, got %d", state.loginCalls)
				}
				if svc.GetCallerIdentityCalls != 2 {
					t.Fatalf("expected 2 GetCallerIdentity calls, got %d", svc.GetCallerIdentityCalls)
				}
				if svc.GetSessionTokenCalls != 1 {
					t.Fatalf("expected 1 GetSessionToken call, got %d", svc.GetSessionTokenCalls)
				}
				if federation.LastCredentials.AccessKeyID != "AKIA_TEMP" {
					t.Fatalf("expected temporary credentials to be used, got %+v", federation.LastCredentials)
				}
				if !strings.Contains(state.stderr.String(), "Credentials are not valid, attempting SSO login...") {
					t.Fatalf("expected SSO fallback stderr output, got: %q", state.stderr.String())
				}
				if !strings.Contains(state.stdout.String(), "No session token found, requesting temporary credentials...") {
					t.Fatalf("expected temporary credentials output, got: %q", state.stdout.String())
				}
			},
		},
		{
			name:    "returns login error",
			profile: "dev-profile",
			setup: func(t *testing.T, svc *mocks.Service, federation *mocks.FederationBuilder, state *workflowState) {
				t.Helper()
				state.loginErr = errors.New("sso failed")
				svc.GetCallerIdentityFunc = func(ctx context.Context, profile string) (awslib.Identity, error) {
					return awslib.Identity{}, errors.New("bad creds")
				}
			},
			wantErr: "SSO login failed: sso failed",
		},
		{
			name:    "returns error when credentials remain invalid after SSO",
			profile: "dev-profile",
			setup: func(t *testing.T, svc *mocks.Service, federation *mocks.FederationBuilder, state *workflowState) {
				t.Helper()
				state.expectedLoginProfile = "dev-profile"
				svc.GetCallerIdentityFunc = func(ctx context.Context, profile string) (awslib.Identity, error) {
					return awslib.Identity{}, errors.New("still invalid")
				}
			},
			wantErr: "credentials still invalid after SSO login: still invalid",
		},
		{
			name:    "returns error when retrieving credentials fails",
			profile: "dev-profile",
			setup: func(t *testing.T, svc *mocks.Service, federation *mocks.FederationBuilder, state *workflowState) {
				t.Helper()
				svc.GetCallerIdentityFunc = func(ctx context.Context, profile string) (awslib.Identity, error) {
					return awslib.Identity{Arn: "arn:aws:iam::123456789012:user/test"}, nil
				}
				svc.RetrieveCredentialsFunc = func(ctx context.Context, profile string) (awslib.Credentials, error) {
					return awslib.Credentials{}, errors.New("retrieve failed")
				}
			},
			wantErr: "failed to retrieve credentials: retrieve failed",
		},
		{
			name:    "returns error when session token request fails",
			profile: "dev-profile",
			setup: func(t *testing.T, svc *mocks.Service, federation *mocks.FederationBuilder, state *workflowState) {
				t.Helper()
				svc.GetCallerIdentityFunc = func(ctx context.Context, profile string) (awslib.Identity, error) {
					return awslib.Identity{Arn: "arn:aws:iam::123456789012:user/test"}, nil
				}
				svc.RetrieveCredentialsFunc = func(ctx context.Context, profile string) (awslib.Credentials, error) {
					return awslib.Credentials{
						AccessKeyID:     "AKIA_LONG",
						SecretAccessKey: "long-secret",
						SessionToken:    "",
					}, nil
				}
				svc.GetSessionTokenFunc = func(ctx context.Context, profile string, durationSeconds int32) (awslib.Credentials, error) {
					return awslib.Credentials{}, errors.New("token request failed")
				}
			},
			wantErr: "failed to get temporary credentials: token request failed",
		},
		{
			name:    "returns error when federation URL build fails",
			profile: "dev-profile",
			setup: func(t *testing.T, svc *mocks.Service, federation *mocks.FederationBuilder, state *workflowState) {
				t.Helper()
				svc.GetCallerIdentityFunc = func(ctx context.Context, profile string) (awslib.Identity, error) {
					return awslib.Identity{Arn: "arn:aws:iam::123456789012:user/test"}, nil
				}
				svc.RetrieveCredentialsFunc = func(ctx context.Context, profile string) (awslib.Credentials, error) {
					return awslib.Credentials{
						AccessKeyID:     "AKIA_TEST",
						SecretAccessKey: "secret",
						SessionToken:    "token",
					}, nil
				}
				federation.BuildConsoleURLFunc = func(ctx context.Context, creds awslib.Credentials, durationSeconds int32) (string, error) {
					return "", errors.New("federation failed")
				}
			},
			wantErr: "failed to build console URL: federation failed",
		},
		{
			name:    "returns error when browser open fails",
			profile: "dev-profile",
			setup: func(t *testing.T, svc *mocks.Service, federation *mocks.FederationBuilder, state *workflowState) {
				t.Helper()
				state.openErr = errors.New("open failed")
				svc.GetCallerIdentityFunc = func(ctx context.Context, profile string) (awslib.Identity, error) {
					return awslib.Identity{Arn: "arn:aws:iam::123456789012:user/test"}, nil
				}
				svc.RetrieveCredentialsFunc = func(ctx context.Context, profile string) (awslib.Credentials, error) {
					return awslib.Credentials{
						AccessKeyID:     "AKIA_TEST",
						SecretAccessKey: "secret",
						SessionToken:    "token",
					}, nil
				}
				federation.BuildConsoleURLFunc = func(ctx context.Context, creds awslib.Credentials, durationSeconds int32) (string, error) {
					return "https://example.com/console-login", nil
				}
			},
			wantErr: "open failed",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := &mocks.Service{
				GetCallerIdentityFunc: func(ctx context.Context, profile string) (awslib.Identity, error) {
					return awslib.Identity{}, fmt.Errorf("unexpected GetCallerIdentity call")
				},
				RetrieveCredentialsFunc: func(ctx context.Context, profile string) (awslib.Credentials, error) {
					return awslib.Credentials{}, fmt.Errorf("unexpected RetrieveCredentials call")
				},
				GetSessionTokenFunc: func(ctx context.Context, profile string, durationSeconds int32) (awslib.Credentials, error) {
					return awslib.Credentials{}, fmt.Errorf("unexpected GetSessionToken call")
				},
			}

			federation := &mocks.FederationBuilder{
				BuildConsoleURLFunc: func(ctx context.Context, creds awslib.Credentials, durationSeconds int32) (string, error) {
					return "", fmt.Errorf("unexpected BuildConsoleURL call")
				},
			}

			state := &workflowState{}
			tc.setup(t, svc, federation, state)

			deps := runDeps{
				awsService: svc,
				federation: federation,
				login: func(profile string) error {
					state.loginCalls++
					state.lastLoginProfile = profile
					if state.expectedLoginProfile != "" && profile != state.expectedLoginProfile {
						t.Fatalf("unexpected profile passed to login: %q", profile)
					}
					return state.loginErr
				},
				open: func(targetURL string) error {
					state.openedURL = targetURL
					return state.openErr
				},
				stdout:          &state.stdout,
				stderr:          &state.stderr,
				sessionDuration: sessionDuration,
			}

			err := runWorkflow(context.Background(), tc.profile, deps)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q but got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tc.assertions != nil {
				tc.assertions(t, svc, federation, state)
			}
		})
	}
}

func TestNewRootCmdProfileResolution(t *testing.T) {
	testCases := []struct {
		name        string
		args        []string
		envProfile  string
		wantProfile string
	}{
		{
			name:        "uses explicit profile flag",
			args:        []string{"--profile", "flag-profile"},
			envProfile:  "env-profile",
			wantProfile: "flag-profile",
		},
		{
			name:        "uses environment profile when flag absent",
			args:        []string{},
			envProfile:  "env-profile",
			wantProfile: "env-profile",
		},
		{
			name:        "uses empty profile when unset",
			args:        []string{},
			envProfile:  "",
			wantProfile: "",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("AWS_PROFILE", tc.envProfile)

			capturedProfile := "__unset__"
			deps := runDeps{
				awsService:      &mocks.Service{},
				federation:      &mocks.FederationBuilder{},
				login:           func(profile string) error { return nil },
				open:            func(targetURL string) error { return nil },
				stdout:          &bytes.Buffer{},
				stderr:          &bytes.Buffer{},
				sessionDuration: sessionDuration,
			}

			root := newRootCmd(deps, func(ctx context.Context, profile string, deps runDeps) error {
				capturedProfile = profile
				return nil
			})
			root.SetArgs(tc.args)

			if err := root.Execute(); err != nil {
				t.Fatalf("unexpected execute error: %v", err)
			}

			if capturedProfile != tc.wantProfile {
				t.Fatalf("expected profile %q, got %q", tc.wantProfile, capturedProfile)
			}
		})
	}
}

func TestNewRootCmdProfileFlagConfigured(t *testing.T) {
	t.Parallel()

	root := NewRootCmd()
	flag := root.Flags().Lookup("profile")
	if flag == nil {
		t.Fatal("expected profile flag to be registered")
	}
	if flag.Shorthand != "p" {
		t.Fatalf("expected shorthand 'p', got %q", flag.Shorthand)
	}
}

func TestSSOLogin(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		profile       string
		runErr        error
		wantArgs      []string
		wantErrSubstr string
	}{
		{
			name:     "without profile",
			profile:  "",
			wantArgs: []string{"sso", "login"},
		},
		{
			name:     "with profile",
			profile:  "dev-profile",
			wantArgs: []string{"sso", "login", "--profile", "dev-profile"},
		},
		{
			name:          "executor error",
			profile:       "dev-profile",
			runErr:        errors.New("exec failed"),
			wantArgs:      []string{"sso", "login", "--profile", "dev-profile"},
			wantErrSubstr: "exec failed",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			executor := &fakeExecutor{runErr: tc.runErr}
			deps := runDeps{
				executor: executor,
				stdin:    strings.NewReader(""),
				stdout:   &bytes.Buffer{},
				stderr:   &bytes.Buffer{},
			}

			err := ssoLogin(tc.profile, deps)
			if tc.wantErrSubstr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErrSubstr) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErrSubstr, err)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(executor.calls) != 1 {
				t.Fatalf("expected 1 executor call, got %d", len(executor.calls))
			}
			call := executor.calls[0]
			if call.method != "run" || call.name != "aws" {
				t.Fatalf("unexpected executor call: %+v", call)
			}
			if strings.Join(call.args, "|") != strings.Join(tc.wantArgs, "|") {
				t.Fatalf("unexpected args: got %v want %v", call.args, tc.wantArgs)
			}
		})
	}
}

func TestOpenBrowser(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		goos          string
		startErr      error
		wantName      string
		wantArgs      []string
		wantErrSubstr string
	}{
		{
			name:     "darwin",
			goos:     "darwin",
			wantName: "open",
			wantArgs: []string{"https://example.com"},
		},
		{
			name:     "linux",
			goos:     "linux",
			wantName: "xdg-open",
			wantArgs: []string{"https://example.com"},
		},
		{
			name:     "windows",
			goos:     "windows",
			wantName: "rundll32",
			wantArgs: []string{"url.dll,FileProtocolHandler", "https://example.com"},
		},
		{
			name:          "unsupported",
			goos:          "plan9",
			wantErrSubstr: "unsupported platform: plan9",
		},
		{
			name:          "start error",
			goos:          "linux",
			startErr:      errors.New("start failed"),
			wantName:      "xdg-open",
			wantArgs:      []string{"https://example.com"},
			wantErrSubstr: "start failed",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			executor := &fakeExecutor{startErr: tc.startErr}
			deps := runDeps{
				executor: executor,
				goos:     tc.goos,
			}

			err := openBrowser("https://example.com", deps)
			if tc.wantErrSubstr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErrSubstr) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErrSubstr, err)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tc.goos == "plan9" {
				if len(executor.calls) != 0 {
					t.Fatalf("expected no executor calls for unsupported platform, got %d", len(executor.calls))
				}
				return
			}

			if len(executor.calls) != 1 {
				t.Fatalf("expected 1 executor call, got %d", len(executor.calls))
			}
			call := executor.calls[0]
			if call.method != "start" || call.name != tc.wantName {
				t.Fatalf("unexpected executor call: %+v", call)
			}
			if strings.Join(call.args, "|") != strings.Join(tc.wantArgs, "|") {
				t.Fatalf("unexpected args: got %v want %v", call.args, tc.wantArgs)
			}
		})
	}
}
