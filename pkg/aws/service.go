package aws

import (
	"context"
	"fmt"

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

type configLoader interface {
	LoadDefaultConfig(ctx context.Context, optFns ...func(*config.LoadOptions) error) (awsv2.Config, error)
}

type defaultConfigLoader struct{}

func (defaultConfigLoader) LoadDefaultConfig(ctx context.Context, optFns ...func(*config.LoadOptions) error) (awsv2.Config, error) {
	return config.LoadDefaultConfig(ctx, optFns...)
}

type stsAPI interface {
	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
	GetSessionToken(ctx context.Context, params *sts.GetSessionTokenInput, optFns ...func(*sts.Options)) (*sts.GetSessionTokenOutput, error)
}

type stsClientFactory interface {
	NewFromConfig(cfg awsv2.Config) stsAPI
}

type defaultSTSClientFactory struct{}

func (defaultSTSClientFactory) NewFromConfig(cfg awsv2.Config) stsAPI {
	return sts.NewFromConfig(cfg)
}

// SDKService is the concrete implementation backed by AWS SDK v2.
type SDKService struct {
	loader     configLoader
	stsFactory stsClientFactory
}

// NewService creates an AWS service implementation that uses AWS SDK v2.
func NewService() *SDKService {
	return newSDKService(defaultConfigLoader{}, defaultSTSClientFactory{})
}

func newSDKService(loader configLoader, stsFactory stsClientFactory) *SDKService {
	return &SDKService{
		loader:     loader,
		stsFactory: stsFactory,
	}
}

func (s *SDKService) loadConfig(ctx context.Context, profile string) (awsv2.Config, error) {
	var opts []func(*config.LoadOptions) error
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}

	cfg, err := s.loader.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return awsv2.Config{}, fmt.Errorf("failed to load AWS config: %w", err)
	}
	return cfg, nil
}

func (s *SDKService) GetCallerIdentity(ctx context.Context, profile string) (Identity, error) {
	cfg, err := s.loadConfig(ctx, profile)
	if err != nil {
		return Identity{}, err
	}

	out, err := s.stsFactory.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return Identity{}, err
	}

	return Identity{Arn: awsv2.ToString(out.Arn)}, nil
}

func (s *SDKService) RetrieveCredentials(ctx context.Context, profile string) (Credentials, error) {
	cfg, err := s.loadConfig(ctx, profile)
	if err != nil {
		return Credentials{}, err
	}

	creds, err := cfg.Credentials.Retrieve(ctx)
	if err != nil {
		return Credentials{}, err
	}

	return Credentials{
		AccessKeyID:     creds.AccessKeyID,
		SecretAccessKey: creds.SecretAccessKey,
		SessionToken:    creds.SessionToken,
	}, nil
}

func (s *SDKService) GetSessionToken(ctx context.Context, profile string, durationSeconds int32) (Credentials, error) {
	cfg, err := s.loadConfig(ctx, profile)
	if err != nil {
		return Credentials{}, err
	}

	out, err := s.stsFactory.NewFromConfig(cfg).GetSessionToken(ctx, &sts.GetSessionTokenInput{
		DurationSeconds: awsv2.Int32(durationSeconds),
	})
	if err != nil {
		return Credentials{}, err
	}

	if out.Credentials == nil {
		return Credentials{}, fmt.Errorf("STS GetSessionToken returned empty credentials")
	}

	return Credentials{
		AccessKeyID:     awsv2.ToString(out.Credentials.AccessKeyId),
		SecretAccessKey: awsv2.ToString(out.Credentials.SecretAccessKey),
		SessionToken:    awsv2.ToString(out.Credentials.SessionToken),
	}, nil
}
