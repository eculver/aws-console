package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"

	awslib "github.com/eculver/aws-console/pkg/aws"
	"github.com/spf13/cobra"
)

const sessionDuration = 43200 // 12 hours (max for federation)

// Executor abstracts command execution for easier testing.
type Executor interface {
	Run(name string, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error
	Start(name string, args []string) error
}

type osExecutor struct{}

func (osExecutor) Run(name string, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	cliCmd := exec.Command(name, args...)
	cliCmd.Stdin = stdin
	cliCmd.Stdout = stdout
	cliCmd.Stderr = stderr
	return cliCmd.Run()
}

func (osExecutor) Start(name string, args []string) error {
	return exec.Command(name, args...).Start()
}

type runDeps struct {
	awsService      awslib.Service
	federation      awslib.FederationURLBuilder
	login           func(string) error
	open            func(string) error
	executor        Executor
	goos            string
	stdin           io.Reader
	stdout          io.Writer
	stderr          io.Writer
	sessionDuration int32
}

type workflowRunner func(ctx context.Context, profile string, deps runDeps) error

// NewRootCmd creates the root CLI command.
func NewRootCmd() *cobra.Command {
	return newRootCmd(defaultRunDeps(), runWorkflow)
}

func newRootCmd(deps runDeps, runner workflowRunner) *cobra.Command {
	var profile string

	rootCmd := &cobra.Command{
		Use:   "aws-console",
		Short: "Open the AWS Console in your browser using current credentials",
		Long: `Authenticates using your AWS credentials and opens the AWS Management Console
in your default web browser. If credentials are expired or missing, it will
attempt to run 'aws sso login' to refresh them.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedProfile := profile
			if resolvedProfile == "" {
				resolvedProfile = os.Getenv("AWS_PROFILE")
			}
			return runner(context.Background(), resolvedProfile, deps)
		},
	}

	rootCmd.Flags().StringVarP(&profile, "profile", "p", "", "AWS profile to use (defaults to AWS_PROFILE env var)")

	return rootCmd
}

// Execute runs the root command.
func Execute() error {
	return NewRootCmd().Execute()
}

func defaultRunDeps() runDeps {
	deps := runDeps{
		awsService:      awslib.NewService(),
		federation:      awslib.NewFederationClient(),
		executor:        osExecutor{},
		goos:            runtime.GOOS,
		stdin:           os.Stdin,
		stdout:          os.Stdout,
		stderr:          os.Stderr,
		sessionDuration: sessionDuration,
	}

	deps.login = func(profile string) error {
		return ssoLogin(profile, deps)
	}
	deps.open = func(targetURL string) error {
		return openBrowser(targetURL, deps)
	}

	return deps
}

func runWorkflow(ctx context.Context, profile string, deps runDeps) error {
	identity, err := deps.awsService.GetCallerIdentity(ctx, profile)
	if err != nil {
		fmt.Fprintln(deps.stderr, "Credentials are not valid, attempting SSO login...")
		if loginErr := deps.login(profile); loginErr != nil {
			return fmt.Errorf("SSO login failed: %w", loginErr)
		}

		identity, err = deps.awsService.GetCallerIdentity(ctx, profile)
		if err != nil {
			return fmt.Errorf("credentials still invalid after SSO login: %w", err)
		}
	}

	fmt.Fprintf(deps.stdout, "Authenticated as: %s\n", identity.Arn)

	creds, err := deps.awsService.RetrieveCredentials(ctx, profile)
	if err != nil {
		return fmt.Errorf("failed to retrieve credentials: %w", err)
	}

	// If no session token (e.g. long-lived IAM user keys), request temporary credentials
	if creds.SessionToken == "" {
		fmt.Fprintln(deps.stdout, "No session token found, requesting temporary credentials...")
		creds, err = deps.awsService.GetSessionToken(ctx, profile, deps.sessionDuration)
		if err != nil {
			return fmt.Errorf("failed to get temporary credentials: %w", err)
		}
	}

	// Build the federated console sign-in URL
	loginURL, err := deps.federation.BuildConsoleURL(ctx, creds, deps.sessionDuration)
	if err != nil {
		return fmt.Errorf("failed to build console URL: %w", err)
	}

	fmt.Fprintln(deps.stdout, "Opening AWS Console in your browser...")
	return deps.open(loginURL)
}

// ssoLogin shells out to the AWS CLI to perform an SSO login.
func ssoLogin(profile string, deps runDeps) error {
	args := []string{"sso", "login"}
	if profile != "" {
		args = append(args, "--profile", profile)
	}

	return deps.executor.Run("aws", args, deps.stdin, deps.stdout, deps.stderr)
}

// openBrowser opens the given URL in the user's default browser.
func openBrowser(targetURL string, deps runDeps) error {
	var command string
	var args []string

	switch deps.goos {
	case "darwin":
		command = "open"
	case "linux":
		command = "xdg-open"
	case "windows":
		command = "rundll32"
		args = []string{"url.dll,FileProtocolHandler"}
	default:
		return fmt.Errorf("unsupported platform: %s", deps.goos)
	}

	args = append(args, targetURL)
	return deps.executor.Start(command, args)
}
