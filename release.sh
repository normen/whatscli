#!/usr/bin/env bash
set -euo pipefail

WORKFLOW_NAME="Release"

usage() {
  cat <<'EOF'
Usage: ./release.sh [version] [--no-watch]

If version is omitted, the script reads VERSION from main.go.
It then creates the git tag, pushes it, and waits for the GitHub Actions
"Release" workflow that publishes assets and updates the Homebrew tap.
EOF
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing required command: $1" >&2
    exit 1
  fi
}

extract_version() {
  sed -n 's/^var VERSION string = "\(v[^"]*\)"$/\1/p' main.go | head -n1
}

wait_for_run() {
  local version="$1"
  local event_name="$2"
  local timeout_seconds=900
  local started_at
  started_at="$(date +%s)"

  echo "Waiting for workflow \"$WORKFLOW_NAME\" ($event_name) for tag $version to start..."
  while true; do
    local run_id
    run_id="$(
      gh run list \
        --workflow "$WORKFLOW_NAME" \
        --limit 20 \
        --json databaseId,headBranch,event,displayTitle \
        --jq ".[] | select(.event == \"$event_name\" and .headBranch == \"$version\") | .databaseId" \
        | head -n1
    )"

    if [ -n "${run_id:-}" ]; then
      echo "Watching workflow run $run_id"
      gh run watch "$run_id" --exit-status
      return
    fi

    if [ $(( $(date +%s) - started_at )) -ge "$timeout_seconds" ]; then
      echo "Timed out waiting for workflow \"$WORKFLOW_NAME\" for tag $version" >&2
      exit 1
    fi

    sleep 5
  done
}

trigger_workflow_dispatch() {
  local version="$1"
  gh workflow run "$WORKFLOW_NAME" --ref "$version" -f "tag=$version"
}

main() {
  require_cmd git
  require_cmd gh

  local version=""
  local watch_release=1

  while [ $# -gt 0 ]; do
    case "$1" in
      -h|--help)
        usage
        exit 0
        ;;
      --no-watch)
        watch_release=0
        ;;
      *)
        if [ -n "$version" ]; then
          echo "Unexpected extra argument: $1" >&2
          usage
          exit 1
        fi
        version="$1"
        ;;
    esac
    shift
  done

  if [ -z "$version" ]; then
    version="$(extract_version)"
  fi

  if [ -z "$version" ]; then
    echo "Could not determine release version from main.go" >&2
    exit 1
  fi

  echo "Preparing release $version"

  git fetch --tags origin

  gh auth status >/dev/null

  local trigger_event="push"
  if git ls-remote --exit-code --tags origin "refs/tags/$version" >/dev/null 2>&1; then
    echo "Tag $version already exists on origin"
    echo "Triggering workflow_dispatch for the existing tag"
    trigger_workflow_dispatch "$version"
    trigger_event="workflow_dispatch"
  else
    if git rev-parse -q --verify "refs/tags/$version" >/dev/null; then
      echo "Tag $version already exists locally, pushing it to origin"
    else
      git tag -a "$version" -m "Release $version"
    fi

    git push origin "refs/tags/$version"
    echo "Pushed tag $version"
    echo "GitHub Actions will build artifacts, create the GitHub release, and update the Homebrew tap."
  fi

  if [ "$watch_release" -eq 1 ]; then
    wait_for_run "$version" "$trigger_event"
  else
    echo "Skipping workflow watch (--no-watch)"
  fi
}

main "$@"
