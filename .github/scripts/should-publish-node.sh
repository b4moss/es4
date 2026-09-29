#!/usr/bin/env bash
# Decide whether @b4moss/es4 should be published from the current HEAD.
# Outputs GitHub Actions-style keys to GITHUB_OUTPUT when set:
#   skip=true|false
#   tag=vX.Y.Z (when not skipped for missing / mismatched tag)
#   version=X.Y.Z
#
# Gate:
# - packages/node/package.json version X.Y.Z must have git tag vX.Y.Z
# - that tag must be HEAD or an ancestor of HEAD (or same tree as HEAD —
#   squash-merge onto release safe)
# - skip when @b4moss/es4@X.Y.Z already exists on the npm registry
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
PKG_DIR="$ROOT/packages/node"

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

PKG_VER="$(node -p "require('${PKG_DIR}/package.json').version")"
TAG="v${PKG_VER}"
emit "version" "$PKG_VER"

if [[ "$PKG_VER" == *-* ]]; then
  skip "packages/node version ${PKG_VER} looks like a prerelease/dev build; skip npm publish."
fi

if ! git rev-parse -q --verify "refs/tags/${TAG}" >/dev/null; then
  skip "No git tag ${TAG} for packages/node version ${PKG_VER}; skip npm publish."
fi

TAG_COMMIT="$(git rev-list -n 1 "${TAG}")"
HEAD_COMMIT="$(git rev-parse HEAD)"
TAG_TREE="$(git rev-parse "${TAG_COMMIT}^{tree}")"
HEAD_TREE="$(git rev-parse "${HEAD_COMMIT}^{tree}")"

if [[ "$TAG_COMMIT" != "$HEAD_COMMIT" ]] &&
  ! git merge-base --is-ancestor "$TAG_COMMIT" "$HEAD_COMMIT" &&
  [[ "$TAG_TREE" != "$HEAD_TREE" ]]; then
  skip "Tag ${TAG} (${TAG_COMMIT}) is not an ancestor of HEAD and trees differ; skip npm publish."
fi

if [[ "$TAG_COMMIT" == "$HEAD_COMMIT" ]] ||
  git merge-base --is-ancestor "$TAG_COMMIT" "$HEAD_COMMIT"; then
  echo "Using tag ${TAG} at ${TAG_COMMIT} (HEAD=${HEAD_COMMIT})."
elif [[ "$TAG_TREE" == "$HEAD_TREE" ]]; then
  echo "Using tag ${TAG} at ${TAG_COMMIT} via matching tree on HEAD=${HEAD_COMMIT} (squash-safe)."
fi
emit "tag" "$TAG"

# npm refuses republish of the same SemVer.
if npm view "@b4moss/es4@${PKG_VER}" version >/dev/null 2>&1; then
  skip "npm already has @b4moss/es4@${PKG_VER}; skip publish (no republish)."
fi

echo "Will publish @b4moss/es4@${PKG_VER}."
emit "skip" "false"
