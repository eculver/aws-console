package aws

import "context"

// Identity captures the principal that authenticated with STS.
type Identity struct {
	Arn string
}

// Credentials are temporary or long-lived AWS credentials.
type Credentials struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
}

// Service handles credential and identity operations against AWS APIs.
type Service interface {
	GetCallerIdentity(ctx context.Context, profile string) (Identity, error)
	RetrieveCredentials(ctx context.Context, profile string) (Credentials, error)
	GetSessionToken(ctx context.Context, profile string, durationSeconds int32) (Credentials, error)
}

// FederationURLBuilder builds a federated console login URL.
type FederationURLBuilder interface {
	BuildConsoleURL(ctx context.Context, creds Credentials, durationSeconds int32) (string, error)
}
