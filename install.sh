#!/bin/sh

set -eu

repository="ahme-dev/chansat"
repository_url="https://github.com/$repository"
install_dir="${CHANSAT_INSTALL_DIR:-$HOME/.local/bin}"

fail() {
	printf 'chansat: %s\n' "$1" >&2
	exit 1
}

command -v curl >/dev/null 2>&1 || fail "curl is required"
command -v tar >/dev/null 2>&1 || fail "tar is required"

case "$(uname -s)" in
	Darwin) os="darwin" ;;
	Linux) os="linux" ;;
	*) fail "unsupported operating system: $(uname -s)" ;;
esac

case "$(uname -m)" in
	arm64 | aarch64) arch="arm64" ;;
	x86_64 | amd64) arch="amd64" ;;
	*) fail "unsupported architecture: $(uname -m)" ;;
esac

requested_version="${CHANSAT_VERSION:-}"
version="$requested_version"
if [ -z "$version" ]; then
	latest_url="$(
		curl -fsSL -o /dev/null -w '%{url_effective}' \
			"$repository_url/releases/latest"
	)"
	tag="${latest_url##*/}"
	[ "$tag" != "latest" ] || fail "could not determine the latest release"
	version="${tag#v}"
else
	version="${version#v}"
fi

installed_binary="$install_dir/chansat"
if [ ! -x "$installed_binary" ] && command -v chansat >/dev/null 2>&1; then
	installed_binary="$(command -v chansat)"
fi

installed_version=""
if [ -x "$installed_binary" ]; then
	installed_version="$("$installed_binary" version 2>/dev/null | awk 'NR == 1 { print $NF }')"
	installed_version="${installed_version#v}"
fi

if [ "$installed_version" = "$version" ]; then
	printf 'chansat %s is already installed at %s\n' "$version" "$installed_binary"
	exit 0
fi

version_is_newer() {
	awk -v candidate="$1" -v installed="$2" '
		BEGIN {
			sub(/-.*/, "", candidate)
			sub(/-.*/, "", installed)
			nc = split(candidate, c, ".")
			ni = split(installed, i, ".")
			n = nc > ni ? nc : ni
			for (part = 1; part <= n; part++) {
				cv = c[part] + 0
				iv = i[part] + 0
				if (cv > iv) exit 0
				if (cv < iv) exit 1
			}
			exit 1
		}
	'
}

if [ -n "$installed_version" ]; then
	if version_is_newer "$version" "$installed_version"; then
		printf 'Updating chansat %s to %s...\n' "$installed_version" "$version"
	elif [ -n "$requested_version" ]; then
		printf 'Installing requested chansat %s over %s...\n' \
			"$version" "$installed_version"
	else
		printf 'Installed chansat %s is newer than release %s; nothing to do.\n' \
			"$installed_version" "$version"
		exit 0
	fi
fi

archive="chansat_${version}_${os}_${arch}.tar.gz"
download_url="$repository_url/releases/download/v${version}"
temporary_dir="$(mktemp -d)"
trap 'rm -rf "$temporary_dir"' EXIT HUP INT TERM

printf 'Downloading chansat %s for %s/%s...\n' "$version" "$os" "$arch"
curl -fsSL "$download_url/$archive" -o "$temporary_dir/$archive"
curl -fsSL "$download_url/checksums.txt" -o "$temporary_dir/checksums.txt"

expected="$(
		awk -v archive="$archive" \
			'$2 == archive || $2 == ("*" archive) { print $1; exit }' \
			"$temporary_dir/checksums.txt"
	)"
[ -n "$expected" ] || fail "checksum not found for $archive"

if command -v sha256sum >/dev/null 2>&1; then
	actual="$(sha256sum "$temporary_dir/$archive" | awk '{ print $1 }')"
elif command -v shasum >/dev/null 2>&1; then
	actual="$(shasum -a 256 "$temporary_dir/$archive" | awk '{ print $1 }')"
else
	fail "sha256sum or shasum is required"
fi
[ "$actual" = "$expected" ] || fail "checksum verification failed"

tar -xzf "$temporary_dir/$archive" -C "$temporary_dir"
mkdir -p "$install_dir"
install -m 0755 "$temporary_dir/chansat" "$install_dir/chansat"

printf 'Installed chansat to %s/chansat\n' "$install_dir"
case ":$PATH:" in
	*":$install_dir:"*) ;;
	*) printf 'Add %s to PATH to run chansat.\n' "$install_dir" ;;
esac
