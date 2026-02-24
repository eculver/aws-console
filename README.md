# aws-console

Opens the AWS Management Console in your default browser using your current AWS credentials.

Falls back to using `aws sso` to refresh credentials when a session doesn't exist which may trigger a web login form. Once the session is valid, it will try to exchange the SSO identity for a pre-authenticated console URL and open it automatically in your browser. Details below.

## How it works

1. Resolves your AWS profile from the `-p`/`--profile` flag or the `AWS_PROFILE` environment variable.
2. Validates credentials by calling STS `GetCallerIdentity`.
3. If credentials are expired or missing, automatically runs `aws sso login` to refresh them.
4. If the credentials are long-lived IAM keys (no session token), requests temporary credentials via STS `GetSessionToken`.
5. Sends the temporary credentials to the [AWS federation endpoint](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_providers_enable-console-custom-url.html) to obtain a sign-in token.
6. Constructs a pre-authenticated console URL and opens it in your browser.

## Quickstart

`aws-console` is designed for AWS SSO users. If you haven't already, configure SSO with:

```bash
aws configure sso
```

This will create a profile entry in `~/.aws/config`. A minimal SSO profile looks like this:

```ini
[profile admin]
sso_session = my-sso
sso_account_id = 123456789012
sso_role_name = AdministratorAccess
region = us-east-1
output = json

[sso-session my-sso]
sso_start_url = https://my-org.awsapps.com/start
sso_region = us-east-1
sso_registration_scopes = sso:account:access
```

Once your profile is configured, open the AWS Console in your browser:

```bash
aws-console -p admin
```

Or set the profile via environment variable:

```bash
export AWS_PROFILE=admin
aws-console
```

If your SSO session has expired, `aws-console` will automatically run `aws sso login` to refresh it before opening the console.

## Install

```bash
go install github.com/eculver/aws-console@latest
```

## Usage

```bash
aws-console [flags]

Flags:
  -p, --profile string   AWS profile to use (defaults to AWS_PROFILE env var)
  -v, --version          Print the current version
  -h, --help             help for aws-console
```

### Examples

```bash
# Use the profile from AWS_PROFILE env var
export AWS_PROFILE=my-profile
aws-console

# Specify a profile explicitly
aws-console -p my-profile

# Print the build version
aws-console --version
```

## Prerequisites

- Go 1.21+ (to build)
- The [AWS CLI](https://aws.amazon.com/cli/) must be installed and on your `PATH` (used for SSO login fallback)
- A configured AWS profile in `~/.aws/config` (SSO, IAM user, or assume-role)

## Contributing

Contributions are welcome! Make sure you have Go 1.21+ installed, then:

```bash
git clone https://github.com/eculver/aws-console.git
cd aws-console
```

Build the project:

```bash
make
```

Run the tests:

```bash
make test
```

Other useful targets:

| Target               | Description                                   |
| -------------------- | --------------------------------------------- |
| `make build`         | Build the `aws-console` binary                |
| `make test`          | Run all tests                                 |
| `make test-release`  | Run release helper script tests               |
| `make coverage`      | Run tests and print coverage summary          |
| `make coverage-html` | Generate and open HTML coverage report        |
| `make install`       | Install via `go install`                      |
| `make fmt`           | Format source files                           |
| `make vet`           | Run `go vet`                                  |
| `make clean`         | Remove the built binary                       |

### Releases

Releases are semver tags (`vMAJOR.MINOR.PATCH`) that trigger the GitHub `Release` workflow.

- Create a release tag with the helper script:

  ```bash
  ./scripts/release.sh major [--push] [--yes]
  ./scripts/release.sh minor [--push] [--yes]
  ./scripts/release.sh bugfix [--push] [--yes]
  ```

  The annotated tag message is set automatically to the new tag name (e.g. `v2.0.0`).

- `--push` pushes the tag to `origin` (which triggers GitHub release publishing).
- `--yes` runs non-interactively (no confirmation prompts).
- Without `--push`, the script reminds you to push the tag manually.

You can also call this through Make:

```bash
make release ARGS='major --push --yes'
```

or with explicit convenience targets:

```bash
make release-major PUSH=1 YES=1
make release-minor PUSH=1 YES=1
make release-bugfix PUSH=1 YES=1
```

The GitHub release notes are generated from commits since the previous semver tag. Conventional commit messages are grouped into:

- Breaking changes
- Features (`feat:`)
- Bug fixes (`fix:`)
- Other changes

You can generate notes locally with:

```bash
./scripts/generate-release-notes.sh --current-tag v1.2.4
```
