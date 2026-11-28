#!/usr/bin/env bash
# Update the THRM flake to the latest upstream stable release.
#
#   scripts/update.sh                    check only (exit 10 when an update is available)
#   scripts/update.sh --update           update version, resolve hashes, build, verify
#   scripts/update.sh --update --force   re-resolve hashes for the packaged version
#   scripts/update.sh --check --json     machine-readable result on stdout
#
# Only stable upstream releases (vX.Y.Z) are tracked; nightly prereleases are
# ignored. The script never commits or pushes; .github/workflows/update.yml
# does that in CI.
#
# Environment: GITHUB_TOKEN is optional (API rate limit / CI token).

set -euo pipefail

ROOT=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
NIX_FILE="$ROOT/nix/thrm.nix"
UPSTREAM_OWNER=TIANLI0
UPSTREAM_REPO=THRM
FAKE_HASH="sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="

mode=check
force=false
json=false

usage() {
  sed -n '2,14p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

while (($#)); do
  case "$1" in
    --check) mode=check ;;
    --update) mode=update ;;
    --force) force=true ;;
    --json) json=true ;;
    -h|--help) usage; exit 0 ;;
    *) printf 'unknown argument: %s\n' "$1" >&2; usage >&2; exit 2 ;;
  esac
  shift
done

log() { printf '%s\n' "$*" >&2; }

for cmd in nix curl sed sort git; do
  command -v "$cmd" >/dev/null 2>&1 || {
    log "error: required command '$cmd' not found"
    exit 1
  }
done

emit_json() { # current latest updateAvailable
  printf '{"current":"%s","latest":"%s","updateAvailable":%s}\n' "$1" "$2" "$3"
}

api_get() {
  local url="https://api.github.com/$1"
  if [[ -n "${GITHUB_TOKEN:-}" ]]; then
    curl -fsSL --retry 3 --retry-delay 2 \
      -H 'Accept: application/vnd.github+json' \
      -H "Authorization: Bearer ${GITHUB_TOKEN}" "$url"
  else
    curl -fsSL --retry 3 --retry-delay 2 \
      -H 'Accept: application/vnd.github+json' "$url"
  fi
}

latest_upstream_version() {
  local tag="" body redirect
  # Offline/pinned override, mostly for testing this script.
  if [[ -n "${THRM_LATEST_VERSION:-}" ]]; then
    tag="v${THRM_LATEST_VERSION#v}"
  fi
  # GitHub redirects /releases/latest to the latest non-prerelease tag.  This
  # needs no token and is not subject to the API rate limit.
  if [[ ! "${tag:-}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    redirect=$(curl -fsS --retry 3 --retry-delay 2 -o /dev/null -w '%{redirect_url}' \
      "https://github.com/${UPSTREAM_OWNER}/${UPSTREAM_REPO}/releases/latest" 2>/dev/null || true)
    tag=$(printf '%s\n' "$redirect" \
      | sed -nE 's|.*/releases/tag/([^/?#]+).*|\1|p' \
      | head -n1)
  fi
  if [[ ! "${tag:-}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    log 'warning: release redirect did not yield a stable tag; trying the GitHub API'
    if body=$(api_get "repos/${UPSTREAM_OWNER}/${UPSTREAM_REPO}/releases/latest" 2>/dev/null); then
      tag=$(printf '%s' "$body" \
        | grep -oE '"tag_name"[[:space:]]*:[[:space:]]*"[^"]*"' \
        | head -n1 \
        | sed -E 's/.*"([^"]*)"$/\1/')
    fi
  fi
  if [[ ! "${tag:-}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    log 'warning: GitHub API unavailable; falling back to git ls-remote'
    tag=$(git ls-remote --tags --refs "https://github.com/${UPSTREAM_OWNER}/${UPSTREAM_REPO}.git" 2>/dev/null \
      | awk '{print $2}' \
      | sed 's|refs/tags/||' \
      | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' \
      | sort -V | tail -n1 || true)
  fi
  if [[ ! "${tag:-}" =~ ^v([0-9]+\.[0-9]+\.[0-9]+)$ ]]; then
    log "error: cannot determine the upstream stable version (tag: ${tag:-none})"
    log 'hint: set THRM_LATEST_VERSION to test against a specific version'
    exit 1
  fi
  printf '%s\n' "${BASH_REMATCH[1]}"
}

current_packaged_version() {
  local version
  version=$(sed -nE 's/^  version \? "([^"]+)",$/\1/p' "$NIX_FILE" | head -n1)
  if [[ -z "$version" ]]; then
    log "error: cannot read version from $NIX_FILE"
    exit 1
  fi
  printf '%s\n' "$version"
}

version_is_newer() { # candidate reference
  [[ "$1" != "$2" && "$(printf '%s\n%s\n' "$1" "$2" | sort -V | tail -n1)" == "$2" ]]
}

replace_default() { # attribute value
  sed -i -E "s|^  $1 \? \"[^\"]*\",$|  $1 ? \"$2\",|" "$NIX_FILE"
}

read_default() {
  sed -nE "s/^  $1 \? \"([^\"]+)\",$/\1/p" "$NIX_FILE" | head -n1
}

set_version() { replace_default version "$1"; }
set_hash() { replace_default "$1" "$2"; }

resolve_hashes() {
  local attempt out drv name got var
  set_hash srcHash "$FAKE_HASH"
  set_hash vendorHash "$FAKE_HASH"
  set_hash frontendDepsHash "$FAKE_HASH"
  for attempt in 1 2 3 4 5 6; do
    log "build attempt $attempt (resolving fixed-output hashes)"
    if out=$(nix build "$ROOT#thrm" --no-link 2>&1); then
      return 0
    fi
    drv=$(printf '%s\n' "$out" \
      | sed -n "s/.*fixed-output derivation '\([^']*\.drv\)'.*/\1/p" \
      | head -n1)
    got=$(printf '%s\n' "$out" \
      | sed -n 's/.*got:[[:space:]]*\(sha256-[A-Za-z0-9+/=]*\).*/\1/p' \
      | head -n1)
    if [[ -z "$drv" || -z "$got" ]]; then
      log 'error: nix build failed for a non-hash reason:'
      printf '%s\n' "$out" | tail -n 40 >&2
      return 1
    fi
    name=$(basename "$drv" .drv)
    case "$name" in
      *-source) var=srcHash ;;
      *-thrm-frontend-deps-*) var=frontendDepsHash ;;
      *-go-modules) var=vendorHash ;;
      *)
        log "error: unexpected fixed-output derivation: $drv"
        return 1
        ;;
    esac
    log "  $var = $got"
    set_hash "$var" "$got"
  done
  log 'error: hashes did not converge after 6 attempts'
  return 1
}

current=$(current_packaged_version)
latest=$(latest_upstream_version)
log "packaged: $current; upstream stable: $latest"

if [[ "$latest" == "$current" ]]; then
  if ! $force; then
    log 'already up to date'
    if $json; then emit_json "$current" "$latest" false; fi
    exit 0
  fi
  target=$current
elif version_is_newer "$current" "$latest"; then
  target=$latest
else
  log "upstream $latest is not newer than packaged $current; nothing to do"
  if $json; then emit_json "$current" "$latest" false; fi
  exit 0
fi

if [[ "$mode" == check ]]; then
  if $json; then emit_json "$current" "$target" true; fi
  log "update available: $current -> $target"
  exit 10
fi

backup=$(mktemp "${NIX_FILE}.XXXXXX")
cp -- "$NIX_FILE" "$backup"
restore_on_failure() {
  local status=$?
  if ((status != 0)); then
    cp -- "$backup" "$NIX_FILE"
    log "restored $NIX_FILE after failure"
  fi
  rm -f -- "$backup"
}
trap restore_on_failure EXIT

if [[ "$target" != "$current" ]]; then
  log "updating version: $current -> $target"
  set_version "$target"
else
  log "refreshing hashes for $current"
fi

resolve_hashes

log 'verifying with nix flake check'
nix flake check "$ROOT"

src_hash=$(read_default srcHash)
vendor_hash=$(read_default vendorHash)
frontend_hash=$(read_default frontendDepsHash)
log "updated to $target"

if $json; then
  printf '{"current":"%s","latest":"%s","updated":true,"version":"%s","srcHash":"%s","vendorHash":"%s","frontendDepsHash":"%s"}\n' \
    "$current" "$target" "$target" "$src_hash" "$vendor_hash" "$frontend_hash"
else
  printf 'updated to %s (srcHash %s, vendorHash %s, frontendDepsHash %s)\n' \
    "$target" "$src_hash" "$vendor_hash" "$frontend_hash"
fi