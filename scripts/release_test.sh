#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RELEASE_SCRIPT="${SCRIPT_DIR}/release.sh"

assert_contains() {
  local haystack="$1"
  local needle="$2"
  if [[ "${haystack}" != *"${needle}"* ]]; then
    echo "Assertion failed: expected output to contain '${needle}'" >&2
    echo "Actual output:" >&2
    echo "${haystack}" >&2
    exit 1
  fi
}

assert_tag_exists() {
  local tag="$1"
  if ! git rev-parse -q --verify "refs/tags/${tag}" >/dev/null 2>&1; then
    echo "Assertion failed: expected tag '${tag}' to exist." >&2
    exit 1
  fi
}

setup_repo() {
  local temp_dir
  temp_dir="$(mktemp -d)"
  cd "${temp_dir}"

  git init >/dev/null
  git config user.name "Release Test"
  git config user.email "release-test@example.com"

  echo "initial" > app.txt
  git add app.txt
  git commit -m "feat: initial commit" >/dev/null
  git tag -a v1.2.3 -m "Release v1.2.3"
}

test_bugfix_bump_creates_annotated_tag() {
  setup_repo

  local output
  output="$("${RELEASE_SCRIPT}" bugfix -m "Release v1.2.4" --yes 2>&1)"

  assert_contains "${output}" "Created tag: v1.2.4"
  assert_contains "${output}" "Push it with: git push origin v1.2.4"
  assert_tag_exists "v1.2.4"

  local tag_message
  tag_message="$(git tag -l --format='%(contents)' v1.2.4)"
  if [[ "${tag_message}" != "Release v1.2.4" ]]; then
    echo "Assertion failed: expected annotated tag message 'Release v1.2.4', got '${tag_message}'" >&2
    exit 1
  fi
}

test_missing_message_fails() {
  setup_repo

  set +e
  local output
  output="$("${RELEASE_SCRIPT}" bugfix --yes 2>&1)"
  local status=$?
  set -e

  if [[ ${status} -eq 0 ]]; then
    echo "Assertion failed: expected command to fail when message is missing." >&2
    exit 1
  fi
  assert_contains "${output}" "Error: release message is required."
}

main() {
  test_bugfix_bump_creates_annotated_tag
  test_missing_message_fails
  echo "release_test.sh: all tests passed"
}

main "$@"
