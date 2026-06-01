#!/bin/sh
# release.sh — tag and push a new release, or show the latest tag.
#
# Usage:
#   ./scripts/release.sh latest          # print the latest release tag
#   ./scripts/release.sh create v1.2.3   # tag + push to trigger the release workflow

set -e

REPO="tomasweigenast/srouter"

cmd="${1}"

case "${cmd}" in
  latest)
    echo "==> Fetching latest release from GitHub..."
    latest=$(curl -sf "https://api.github.com/repos/${REPO}/releases/latest" \
      | grep '"tag_name"' \
      | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
    if [ -z "${latest}" ]; then
      echo "  No releases found."
    else
      echo "  Latest release: ${latest}"
    fi
    ;;

  create)
    tag="${2}"
    if [ -z "${tag}" ]; then
      echo "ERROR: usage: $0 create <tag>  (e.g. $0 create v1.2.3)" >&2
      exit 1
    fi
    # Validate semver-style tag
    case "${tag}" in
      v[0-9]*) ;;
      *) echo "ERROR: tag must start with 'v' (e.g. v1.2.3)" >&2; exit 1 ;;
    esac
    echo "==> Creating release ${tag}..."
    git tag "${tag}"
    git push origin "${tag}"
    echo "  Tag pushed. GitHub Actions will build and publish the release."
    echo "  https://github.com/${REPO}/actions"
    ;;

  *)
    echo "Usage:"
    echo "  $0 latest              print the latest published release tag"
    echo "  $0 create <tag>        create and push a git tag to trigger a release"
    exit 1
    ;;
esac
