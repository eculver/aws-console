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
  ./scripts/release.sh major -m "Release v2.0.0" [--push] [--yes]
  ./scripts/release.sh minor -m "Release v1.3.0" [--push] [--yes]
  ./scripts/release.sh bugfix -m "Release v1.2.4" [--push] [--yes]
  ```

- `--push` pushes the tag to `origin` (which triggers GitHub release publishing).
- `--yes` runs non-interactively (no confirmation prompts).
- Without `--push`, the script reminds you to push the tag manually.

You can also call this through Make:

```bash
make release ARGS='major -m "Release v2.0.0" --push --yes'
```

or with explicit convenience targets:

```bash
make release-major MESSAGE="Release v2.0.0" PUSH=1 YES=1
make release-minor MESSAGE="Release v1.3.0" PUSH=1 YES=1
make release-bugfix MESSAGE="Release v1.2.4" PUSH=1 YES=1
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
