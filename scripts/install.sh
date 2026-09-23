#!/bin/sh
# Downloads and installs a checksum-verified pvectl release for this OS/arch.
#   curl -fsSL https://raw.githubusercontent.com/davegallant/pvectl/main/scripts/install.sh | sh
#   PVECTL_VERSION=v0.3.0 INSTALL_DIR="$HOME/.local/bin" sh install.sh
set -eu

repo="davegallant/pvectl"
install_dir="${INSTALL_DIR:-/usr/bin}"
release="${PVECTL_VERSION-latest}"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
  linux | darwin) ;;
  *) echo "pvectl: unsupported OS: $os" >&2; exit 1 ;;
esac
arch=$(uname -m)
case "$arch" in
  x86_64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *)
    echo "pvectl: unsupported architecture: $arch" >&2
    exit 1
    ;;
esac

case "$release" in
  latest) release_path=latest ;;
  *[!A-Za-z0-9._-]* | '')
    echo "pvectl: invalid PVECTL_VERSION: $release" >&2
    exit 1
    ;;
  *) release_path="tags/$release" ;;
esac

release_json=$(curl -fsSL "https://api.github.com/repos/${repo}/releases/${release_path}")
urls=$(printf '%s\n' "$release_json" \
  | grep -o '"browser_download_url"[[:space:]]*:[[:space:]]*"[^"]*"' \
  | cut -d '"' -f 4 || true)
archive_url=$(printf '%s\n' "$urls" \
  | grep -E "/pvectl_[^/]*_${os}_${arch}\.tar\.gz$" | head -n 1 || true)
checksums_url=$(printf '%s\n' "$urls" | grep '/checksums\.txt$' | head -n 1 || true)

if [ -z "$archive_url" ]; then
  echo "pvectl: no release asset found for ${os}_${arch}" >&2
  exit 1
fi
if [ -z "$checksums_url" ]; then
  echo "pvectl: release is missing checksums.txt" >&2
  exit 1
fi

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT
archive_name=${archive_url##*/}
curl -fsSL -o "$tmpdir/$archive_name" "$archive_url"
curl -fsSL -o "$tmpdir/checksums.txt" "$checksums_url"

expected=$(awk -v file="$archive_name" '$2 == file { print $1 }' "$tmpdir/checksums.txt")
case "$expected" in
  *[!0-9A-Fa-f]* | '') echo "pvectl: missing or invalid checksum for $archive_name" >&2; exit 1 ;;
esac
if [ "$(printf '%s' "$expected" | wc -c | tr -d ' ')" -ne 64 ]; then
  echo "pvectl: invalid checksum length for $archive_name" >&2
  exit 1
fi
if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$tmpdir/$archive_name" | cut -d ' ' -f 1)
elif command -v shasum >/dev/null 2>&1; then
  actual=$(shasum -a 256 "$tmpdir/$archive_name" | cut -d ' ' -f 1)
else
  echo "pvectl: need sha256sum or shasum to verify the release" >&2
  exit 1
fi
if [ "$actual" != "$expected" ]; then
  echo "pvectl: checksum mismatch for $archive_name; refusing to install" >&2
  exit 1
fi

tar -xzf "$tmpdir/$archive_name" -C "$tmpdir" pvectl

sudo_cmd=""
if [ ! -d "$install_dir" ]; then
  if ! mkdir -p "$install_dir" 2>/dev/null; then
    sudo mkdir -p "$install_dir"
  fi
fi
if [ ! -w "$install_dir" ]; then
  sudo_cmd=sudo
fi

$sudo_cmd install -m 755 "$tmpdir/pvectl" "$install_dir/pvectl"
echo "pvectl installed to $install_dir/pvectl"
