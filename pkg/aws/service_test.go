package aws

import (
	"context"
	"errors"
	"strings"
	"testing"

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"
)

type fakeConfigLoader struct {
	cfg awsv2.Config
	err error
}

func (f fakeConfigLoader) LoadDefaultConfig(ctx context.Context, optFns ...func(*config.LoadOptions) error) (awsv2.Config, error) {
	if f.err != nil {
		return awsv2.Config{}, f.err
	}
	return f.cfg, nil
}

type fakeSTS struct {
	getCallerIdentityOutput *sts.GetCallerIdentityOutput
	getCallerIdentityErr    error
	getSessionTokenOutput   *sts.GetSessionTokenOutput
	getSessionTokenErr      error
}

func (f fakeSTS) GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	if f.getCallerIdentityErr != nil {
		return nil, f.getCallerIdentityErr
	}
	return f.getCallerIdentityOutput, nil
}

func (f fakeSTS) GetSessionToken(ctx context.Context, params *sts.GetSessionTokenInput, optFns ...func(*sts.Options)) (*sts.GetSessionTokenOutput, error) {
	if f.getSessionTokenErr != nil {
		return nil, f.getSessionTokenErr
	}
	return f.getSessionTokenOutput, nil
}

type fakeSTSFactory struct {
	client stsAPI
}

func (f fakeSTSFactory) NewFromConfig(cfg awsv2.Config) stsAPI {
	return f.client
}

type failingCredentialsProvider struct {
	err error
}

func (f failingCredentialsProvider) Retrieve(ctx context.Context) (awsv2.Credentials, error) {
	return awsv2.Credentials{}, f.err
}

func TestSDKServiceGetCallerIdentity(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		loader        configLoader
		stsClient     stsAPI
		wantArn       string
		wantErrSubstr string
	}{
		{
			name:   "success",
			loader: fakeConfigLoader{cfg: awsv2.Config{}},
			stsClient: fakeSTS{
				getCallerIdentityOutput: &sts.GetCallerIdentityOutput{
					Arn: awsv2.String("arn:aws:iam::123456789012:user/test"),
				},
			},
			wantArn: "arn:aws:iam::123456789012:user/test",
		},
		{
			name:          "config load failure",
			loader:        fakeConfigLoader{err: errors.New("load failed")},
			stsClient:     fakeSTS{},
			wantErrSubstr: "failed to load AWS config: load failed",
		},
		{
			name:          "sts failure",
			loader:        fakeConfigLoader{cfg: awsv2.Config{}},
			stsClient:     fakeSTS{getCallerIdentityErr: errors.New("sts failed")},
			wantErrSubstr: "sts failed",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := newSDKService(tc.loader, fakeSTSFactory{client: tc.stsClient})
			identity, err := svc.GetCallerIdentity(context.Background(), "test-profile")

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
				t.Fatalf("GetCallerIdentity returned error: %v", err)
			}

			if identity.Arn != tc.wantArn {
				t.Fatalf("unexpected ARN: %q", identity.Arn)
			}
		})
	}
}

func TestSDKServiceRetrieveCredentials(t *testing.T) {
	t.Parallel()

	successCfg := awsv2.Config{
		Credentials: awsv2.NewCredentialsCache(credentials.StaticCredentialsProvider{
			Value: awsv2.Credentials{
				AccessKeyID:     "AKIA_TEST",
				SecretAccessKey: "secret",
				SessionToken:    "token",
			},
		}),
	}

	failedRetrieveCfg := awsv2.Config{
		Credentials: awsv2.NewCredentialsCache(failingCredentialsProvider{
			err: errors.New("retrieve failed"),
		}),
	}

	testCases := []struct {
		name          string
		loader        configLoader
		wantCreds     Credentials
		wantErrSubstr string
	}{
		{
			name:   "success",
			loader: fakeConfigLoader{cfg: successCfg},
			wantCreds: Credentials{
				AccessKeyID:     "AKIA_TEST",
				SecretAccessKey: "secret",
				SessionToken:    "token",
			},
		},
		{
			name:          "config load failure",
			loader:        fakeConfigLoader{err: errors.New("load failed")},
			wantErrSubstr: "failed to load AWS config: load failed",
		},
		{
			name:          "credentials retrieval failure",
			loader:        fakeConfigLoader{cfg: failedRetrieveCfg},
			wantErrSubstr: "retrieve failed",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := newSDKService(tc.loader, fakeSTSFactory{client: fakeSTS{}})
			creds, err := svc.RetrieveCredentials(context.Background(), "test-profile")

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
				t.Fatalf("RetrieveCredentials returned error: %v", err)
			}

			if creds != tc.wantCreds {
				t.Fatalf("unexpected credentials: %+v", creds)
			}
		})
	}
}

func TestSDKServiceGetSessionToken(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		loader        configLoader
		stsClient     stsAPI
		wantCreds     Credentials
		wantErrSubstr string
	}{
		{
			name:   "success",
			loader: fakeConfigLoader{cfg: awsv2.Config{}},
			stsClient: fakeSTS{
				getSessionTokenOutput: &sts.GetSessionTokenOutput{
					Credentials: &ststypes.Credentials{
						AccessKeyId:     awsv2.String("AKIA_TEMP"),
						SecretAccessKey: awsv2.String("temp-secret"),
						SessionToken:    awsv2.String("temp-token"),
					},
				},
			},
			wantCreds: Credentials{
				AccessKeyID:     "AKIA_TEMP",
				SecretAccessKey: "temp-secret",
				SessionToken:    "temp-token",
			},
		},
		{
			name:          "config load failure",
			loader:        fakeConfigLoader{err: errors.New("load failed")},
			stsClient:     fakeSTS{},
			wantErrSubstr: "failed to load AWS config: load failed",
		},
		{
			name:          "sts failure",
			loader:        fakeConfigLoader{cfg: awsv2.Config{}},
			stsClient:     fakeSTS{getSessionTokenErr: errors.New("sts failed")},
			wantErrSubstr: "sts failed",
		},
		{
			name:          "empty credentials from sts",
			loader:        fakeConfigLoader{cfg: awsv2.Config{}},
			stsClient:     fakeSTS{getSessionTokenOutput: &sts.GetSessionTokenOutput{}},
			wantErrSubstr: "STS GetSessionToken returned empty credentials",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := newSDKService(tc.loader, fakeSTSFactory{client: tc.stsClient})
			creds, err := svc.GetSessionToken(context.Background(), "test-profile", 3600)

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
				t.Fatalf("GetSessionToken returned error: %v", err)
			}

			if creds != tc.wantCreds {
				t.Fatalf("unexpected credentials: %+v", creds)
			}
		})
	}
}
