#!/usr/bin/env bash
# Decide whether the Go module should be "published" from the current HEAD.
# Nested module tag convention: packages/go/vX.Y.Z
# (module path: github.com/b4moss/es4/packages/go)
#
# Outputs (GITHUB_OUTPUT when set):
#   skip=true|false
#   tag=packages/go/vX.Y.Z
#   version=X.Y.Z
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
PKG_DIR="$ROOT/packages/go"
VERSION_FILE="$PKG_DIR/VERSION"
OUT="${GITHUB_OUTPUT:-/dev/stdout}"

emit() {
  local key="$1"
  local value="$2"
  if [[ "${GITHUB_OUTPUT:-}" ]]; then
    echo "${key}=${value}" >>"$GITHUB_OUTPUT"
  else
    echo "${key}=${value}"
  fi
}

skip() {
  local reason="$1"
  echo "$reason"
  emit "skip" "true"
  exit 0
}

if [[ ! -f "$VERSION_FILE" ]]; then
  skip "No packages/go/VERSION; skip Go publish."
fi

PKG_VER="$(tr -d '[:space:]' <"$VERSION_FILE")"
if [[ -z "$PKG_VER" ]]; then
  skip "packages/go/VERSION is empty; skip Go publish."
fi

TAG="packages/go/v${PKG_VER}"
emit "version" "$PKG_VER"
emit "tag" "$TAG"

if ! git rev-parse -q --verify "refs/tags/${TAG}" >/dev/null; then
  skip "No git tag ${TAG} for packages/go version ${PKG_VER}; skip Go publish."
fi

TAG_COMMIT="$(git rev-list -n 1 "${TAG}")"
HEAD_COMMIT="$(git rev-parse HEAD)"
if [[ "$TAG_COMMIT" != "$HEAD_COMMIT" ]] &&
  ! git merge-base --is-ancestor "$TAG_COMMIT" "$HEAD_COMMIT"; then
  skip "Tag ${TAG} (${TAG_COMMIT}) is not an ancestor of HEAD; skip Go publish."
fi

echo "Using tag ${TAG} at ${TAG_COMMIT} (HEAD=${HEAD_COMMIT})."

# Do not skip when a GitHub Release already exists (release-on-tag may race).
# Create Release is idempotent; Ping must still run so proxy.golang.org indexes the module.

echo "Will publish Go module at ${TAG}."
emit "skip" "false"
