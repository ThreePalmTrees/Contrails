#!/bin/sh
set -e

# Get bump type from argument (default: minor)
BUMP="${1:-minor}"

case "$BUMP" in
  patch|minor|major) ;;
  rc)
    echo "Triggering release candidate build on main..."
    gh workflow run rc.yml --ref main
    echo "RC build dispatched. Check: gh run list --workflow=rc.yml"
    exit 0
    ;;
  *)
    echo "Usage: $0 [patch|minor|major|rc]" >&2
    exit 1
    ;;
esac

# Get latest semver tag, default to v0.0.0
LATEST=$(git describe --tags --abbrev=0 --match 'v[0-9]*.[0-9]*.[0-9]*' 2>/dev/null || echo "v0.0.0")

# Parse version components
VERSION="${LATEST#v}"
MAJOR=$(echo "$VERSION" | cut -d. -f1)
MINOR=$(echo "$VERSION" | cut -d. -f2)
PATCH=$(echo "$VERSION" | cut -d. -f3)

# Increment
case "$BUMP" in
  major) MAJOR=$((MAJOR + 1)); MINOR=0; PATCH=0 ;;
  minor) MINOR=$((MINOR + 1)); PATCH=0 ;;
  patch) PATCH=$((PATCH + 1)) ;;
esac

NEW_TAG="v${MAJOR}.${MINOR}.${PATCH}"

git tag "$NEW_TAG"
git push origin "$NEW_TAG"

echo "$NEW_TAG"
