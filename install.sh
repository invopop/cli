#!/bin/sh
#
# Install the Invopop CLI from the latest GitHub release.
#
#   curl -fsSL https://raw.githubusercontent.com/invopop/cli/main/install.sh | sh
#
# Environment:
#   INVOPOP_VERSION      release tag to install, e.g. v0.3.0 (default: latest)
#   INVOPOP_INSTALL_DIR  where to put the binary (default: ~/.local/bin)
#
# Files fetched with curl are not quarantined on macOS, so the unsigned
# binary runs without a Gatekeeper prompt.

set -eu

repo="invopop/cli"
install_dir="${INVOPOP_INSTALL_DIR:-$HOME/.local/bin}"

fail() {
	echo "invopop install: $*" >&2
	exit 1
}

need() {
	command -v "$1" >/dev/null 2>&1 || fail "$1 is required"
}

need curl
need tar
need uname

case "$(uname -s)" in
Darwin) os="darwin" ;;
Linux) os="linux" ;;
*) fail "unsupported OS $(uname -s); download a release from https://github.com/$repo/releases" ;;
esac

case "$(uname -m)" in
x86_64 | amd64) arch="amd64" ;;
arm64 | aarch64) arch="arm64" ;;
*) fail "unsupported architecture $(uname -m)" ;;
esac

# The latest release redirects to its tag, which saves an API call that
# would count against GitHub's rate limit.
tag="${INVOPOP_VERSION:-}"
if [ -z "$tag" ]; then
	url=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$repo/releases/latest") ||
		fail "could not find the latest release"
	tag="${url##*/}"
fi
case "$tag" in
v*) ;;
*) fail "unexpected release tag '$tag'" ;;
esac

archive="invopop.${tag#v}_${os}_${arch}.tar.gz"
base="https://github.com/$repo/releases/download/$tag"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "Downloading invopop $tag for $os/$arch..."
curl -fsSL -o "$tmp/$archive" "$base/$archive" || fail "could not download $base/$archive"
curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt" || fail "could not download checksums"

want=$(awk -v f="$archive" '$2 == f { print $1 }' "$tmp/checksums.txt")
[ -n "$want" ] || fail "no checksum for $archive"
if command -v sha256sum >/dev/null 2>&1; then
	got=$(sha256sum "$tmp/$archive" | awk '{ print $1 }')
else
	got=$(shasum -a 256 "$tmp/$archive" | awk '{ print $1 }')
fi
[ "$got" = "$want" ] || fail "checksum mismatch for $archive"

tar -xzf "$tmp/$archive" -C "$tmp"
bin=$(find "$tmp" -type f -name invopop | head -n 1)
[ -n "$bin" ] || fail "invopop binary not found in $archive"

mkdir -p "$install_dir"
install -m 0755 "$bin" "$install_dir/invopop"
echo "Installed invopop $tag to $install_dir/invopop"

case ":$PATH:" in
*":$install_dir:"*) ;;
*)
	echo
	echo "$install_dir is not on your PATH. Add it with:"
	echo "  export PATH=\"$install_dir:\$PATH\""
	;;
esac
