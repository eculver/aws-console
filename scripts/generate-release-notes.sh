#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  ./scripts/generate-release-notes.sh --current-tag vX.Y.Z [--repo-url URL] [--output FILE]

Options:
  --current-tag  Current release tag (required)
  --repo-url     Repository URL used for links (default: inferred from origin)
  --output       Output markdown file path (default: RELEASE_NOTES.md)
  -h, --help     Show this help text
EOF
}

require_git_repo() {
  if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    echo "Error: must be run inside a git repository." >&2
    exit 1
  fi
}

infer_repo_url() {
  local remote_url
  remote_url="$(git config --get remote.origin.url || true)"
  if [[ -z "${remote_url}" ]]; then
    echo "https://github.com/unknown/unknown"
    return
  fi

  # Supports both git@github.com:owner/repo.git and https://github.com/owner/repo.git.
  remote_url="${remote_url%.git}"
  if [[ "${remote_url}" =~ ^git@github\.com:(.+)$ ]]; then
    echo "https://github.com/${BASH_REMATCH[1]}"
    return
  fi

  echo "${remote_url}"
}

linkify_pr_refs() {
  local subject="$1"
  local repo_url="$2"
  # Replace (#NNN) with ([#NNN](repo_url/pull/NNN)); no-op when no PR ref is present.
  echo "${subject}" | sed -E 's|\(#([0-9]+)\)|([#\1]('"${repo_url}"'\/pull\/\1))|g'
}

parse_semver() {
  local tag="$1"
  if [[ "${tag}" =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
    echo "${BASH_REMATCH[1]} ${BASH_REMATCH[2]} ${BASH_REMATCH[3]}"
    return 0
  fi
  return 1
}

main() {
  require_git_repo

  local current_tag=""
  local repo_url=""
  local output="RELEASE_NOTES.md"

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --current-tag)
        if [[ $# -lt 2 ]]; then
          echo "Error: --current-tag requires a value." >&2
          exit 1
        fi
        current_tag="$2"
        shift 2
        ;;
      --repo-url)
        if [[ $# -lt 2 ]]; then
          echo "Error: --repo-url requires a value." >&2
          exit 1
        fi
        repo_url="$2"
        shift 2
        ;;
      --output)
        if [[ $# -lt 2 ]]; then
          echo "Error: --output requires a value." >&2
          exit 1
        fi
        output="$2"
        shift 2
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

  if [[ -z "${current_tag}" ]]; then
    echo "Error: --current-tag is required." >&2
    usage
    exit 1
  fi

  if [[ -z "${repo_url}" ]]; then
    repo_url="$(infer_repo_url)"
  fi

  local prev_tag
  prev_tag="$(git tag --sort=-v:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | awk -v cur="${current_tag}" '$0 != cur { print; exit }')"

  local range="${current_tag}"
  if [[ -n "${prev_tag}" ]]; then
    range="${prev_tag}..${current_tag}"
  fi

  local release_kind="initial"
  local prev_major prev_minor prev_patch
  local cur_major cur_minor cur_patch
  if [[ -n "${prev_tag}" ]] && parse_semver "${prev_tag}" >/dev/null; then
    read -r prev_major prev_minor prev_patch < <(parse_semver "${prev_tag}")
    if parse_semver "${current_tag}" >/dev/null; then
      read -r cur_major cur_minor cur_patch < <(parse_semver "${current_tag}")
    else
      cur_major="${prev_major}"
      cur_minor="${prev_minor}"
      cur_patch="${prev_patch}"
    fi

    if (( cur_major > prev_major )); then
      release_kind="major"
    elif (( cur_minor > prev_minor )); then
      release_kind="minor"
    elif (( cur_patch > prev_patch )); then
      release_kind="bugfix"
    else
      release_kind="non-sequential"
    fi
  fi

  {
    echo "## Release ${current_tag}"
    echo
    if [[ -n "${prev_tag}" ]]; then
      echo "- Date: $(date +%Y-%m-%d)"
      echo "- Previous version: ${prev_tag}"
      echo "- Release type: ${release_kind}"
      echo "- Compare: ${repo_url}/compare/${prev_tag}...${current_tag}"
    else
      echo "- Date: $(date +%Y-%m-%d)"
      echo "- Previous version: none"
      echo "- Release type: initial"
    fi
    echo
  } > "${output}"

  local commit_lines=()
  while IFS= read -r line; do
    commit_lines+=("${line}")
  done < <(git log --pretty=format:'%s|%h' "${range}")
  if [[ ${#commit_lines[@]} -eq 0 ]]; then
    echo "No commits found in range ${range}" >> "${output}"
    echo "Wrote ${output}"
    exit 0
  fi

  local breaking=""
  local features=""
  local fixes=""
  local others=""

  for line in "${commit_lines[@]}"; do
    local subject sha item header linked_subject
    subject="${line%%|*}"
    sha="${line##*|}"
    linked_subject="$(linkify_pr_refs "${subject}" "${repo_url}")"
    item="- ${linked_subject} ([${sha}](${repo_url}/commit/${sha}))"
    header="${subject%%:*}"

    if [[ "${subject}" == *"BREAKING CHANGE"* ]] || [[ "${header}" == *"!" ]]; then
      breaking+="${item}"$'\n'
    elif [[ "${subject}" == feat:* ]] || [[ "${subject}" == feat\(*\):* ]]; then
      features+="${item}"$'\n'
    elif [[ "${subject}" == fix:* ]] || [[ "${subject}" == fix\(*\):* ]]; then
      fixes+="${item}"$'\n'
    else
      others+="${item}"$'\n'
    fi
  done

  if [[ -n "${breaking}" ]]; then
    {
      echo "### Breaking changes"
      echo
      printf "%s" "${breaking}"
      echo
    } >> "${output}"
  fi

  if [[ -n "${features}" ]]; then
    {
      echo "### Features"
      echo
      printf "%s" "${features}"
      echo
    } >> "${output}"
  fi

  if [[ -n "${fixes}" ]]; then
    {
      echo "### Bug fixes"
      echo
      printf "%s" "${fixes}"
      echo
    } >> "${output}"
  fi

  if [[ -n "${others}" ]]; then
    {
      echo "### Other changes"
      echo
      printf "%s" "${others}"
      echo
    } >> "${output}"
  fi

  echo "Wrote ${output}"
}

main "$@"
