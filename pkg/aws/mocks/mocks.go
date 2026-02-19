package mocks

import (
	"context"
	"fmt"

	awslib "github.com/eculver/aws-console/pkg/aws"
)

type Service struct {
	GetCallerIdentityFunc   func(ctx context.Context, profile string) (awslib.Identity, error)
	RetrieveCredentialsFunc func(ctx context.Context, profile string) (awslib.Credentials, error)
	GetSessionTokenFunc     func(ctx context.Context, profile string, durationSeconds int32) (awslib.Credentials, error)

	GetCallerIdentityCalls   int
	RetrieveCredentialsCalls int
	GetSessionTokenCalls     int
}

func (m *Service) GetCallerIdentity(ctx context.Context, profile string) (awslib.Identity, error) {
	m.GetCallerIdentityCalls++
	if m.GetCallerIdentityFunc == nil {
		return awslib.Identity{}, fmt.Errorf("GetCallerIdentityFunc is not set")
	}
	return m.GetCallerIdentityFunc(ctx, profile)
}

func (m *Service) RetrieveCredentials(ctx context.Context, profile string) (awslib.Credentials, error) {
	m.RetrieveCredentialsCalls++
	if m.RetrieveCredentialsFunc == nil {
		return awslib.Credentials{}, fmt.Errorf("RetrieveCredentialsFunc is not set")
	}
	return m.RetrieveCredentialsFunc(ctx, profile)
}

func (m *Service) GetSessionToken(ctx context.Context, profile string, durationSeconds int32) (awslib.Credentials, error) {
	m.GetSessionTokenCalls++
	if m.GetSessionTokenFunc == nil {
		return awslib.Credentials{}, fmt.Errorf("GetSessionTokenFunc is not set")
	}
	return m.GetSessionTokenFunc(ctx, profile, durationSeconds)
}

type FederationBuilder struct {
	BuildConsoleURLFunc func(ctx context.Context, creds awslib.Credentials, durationSeconds int32) (string, error)

	BuildConsoleURLCalls int
	LastCredentials      awslib.Credentials
	LastDurationSeconds  int32
}

func (m *FederationBuilder) BuildConsoleURL(ctx context.Context, creds awslib.Credentials, durationSeconds int32) (string, error) {
	m.BuildConsoleURLCalls++
	m.LastCredentials = creds
	m.LastDurationSeconds = durationSeconds

	if m.BuildConsoleURLFunc == nil {
		return "", fmt.Errorf("BuildConsoleURLFunc is not set")
	}

	return m.BuildConsoleURLFunc(ctx, creds, durationSeconds)
}
