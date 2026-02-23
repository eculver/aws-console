#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  ./scripts/release.sh major|minor|bugfix -m "Message" [--push|-p] [--yes|-y]

Options:
  -m, --message  Annotated tag message (required)
  -p, --push     Push the created tag to origin
  -y, --yes      Skip confirmation prompts
  -h, --help     Show this help text
EOF
}

require_git_repo() {
  if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    echo "Error: must be run inside a git repository." >&2
    exit 1
  fi
}

latest_tag() {
  git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0"
}

parse_semver() {
  local tag="$1"
  if [[ ! "$tag" =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
    echo "Error: latest tag '$tag' is not semver in the form vMAJOR.MINOR.PATCH." >&2
    exit 1
  fi
  echo "${BASH_REMATCH[1]} ${BASH_REMATCH[2]} ${BASH_REMATCH[3]}"
}

confirm() {
  local prompt="$1"
  local answer
  read -r -p "$prompt [y/N]: " answer
  case "$answer" in
    y|Y|yes|YES) return 0 ;;
    *) return 1 ;;
  esac
}

main() {
  require_git_repo

  if [[ $# -lt 1 ]]; then
    usage
    exit 1
  fi

  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
  esac

  local bump_type="$1"
  shift

  local message=""
  local do_push=false
  local assume_yes=false

  while [[ $# -gt 0 ]]; do
    case "$1" in
      -m|--message)
        if [[ $# -lt 2 ]]; then
          echo "Error: $1 requires a value." >&2
          exit 1
        fi
        message="$2"
        shift 2
        ;;
      -p|--push)
        do_push=true
        shift
        ;;
      -y|--yes)
        assume_yes=true
        shift
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        echo "Error: unknown argument '$1'." >&2
        usage
        exit 1
        ;;
    esac
  done

  if [[ -z "$message" ]]; then
    echo "Error: release message is required." >&2
    usage
    exit 1
  fi

  local current_tag
  current_tag="$(latest_tag)"

  local major minor patch
  read -r major minor patch < <(parse_semver "$current_tag")

  case "$bump_type" in
    major)
      major=$((major + 1))
      minor=0
      patch=0
      ;;
    minor)
      minor=$((minor + 1))
      patch=0
      ;;
    bugfix)
      patch=$((patch + 1))
      ;;
    *)
      echo "Error: bump type must be one of: major, minor, bugfix." >&2
      usage
      exit 1
      ;;
  esac

  local new_tag="v${major}.${minor}.${patch}"
  local head_sha
  head_sha="$(git rev-parse --short HEAD)"

  if git rev-parse -q --verify "refs/tags/${new_tag}" >/dev/null 2>&1; then
    echo "Error: tag ${new_tag} already exists." >&2
    exit 1
  fi

  echo "Current tag: ${current_tag}"
  echo "Next tag:    ${new_tag}"
  echo "Commit:      ${head_sha}"
  echo "Message:     ${message}"
  if [[ "$do_push" == true ]]; then
    echo "Push:        yes (origin ${new_tag})"
  else
    echo "Push:        no"
  fi

  if [[ "$assume_yes" != true ]]; then
    if ! confirm "Create annotated tag ${new_tag}?"; then
      echo "Aborted."
      exit 1
    fi
  fi

  git tag -a "${new_tag}" -m "${message}"
  echo "Created tag: ${new_tag}"

  if [[ "$do_push" == true ]]; then
    if [[ "$assume_yes" != true ]]; then
      if ! confirm "Push ${new_tag} to origin?"; then
        echo "Tag created locally and not pushed."
        echo "Push manually with: git push origin ${new_tag}"
        exit 0
      fi
    fi
    git push origin "${new_tag}"
    echo "Pushed tag: ${new_tag}"
  else
    echo "Tag was created locally."
    echo "Push it with: git push origin ${new_tag}"
  fi
}

main "$@"
