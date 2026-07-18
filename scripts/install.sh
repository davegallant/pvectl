#!/bin/sh
# Downloads and installs the latest pvectl release for the current OS/arch.
#   curl -fsSL https://raw.githubusercontent.com/davegallant/pvectl/main/scripts/install.sh | sh
set -eu

repo="davegallant/pvectl"
install_dir="${INSTALL_DIR:-/usr/bin}"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case "$arch" in
  x86_64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *)
    echo "pvectl: unsupported architecture: $arch" >&2
    exit 1
    ;;
esac

url=$(curl -fsSL "https://api.github.com/repos/${repo}/releases/latest" \
  | grep -o "\"browser_download_url\": *\"[^\"]*${os}_${arch}\.tar\.gz\"" \
  | cut -d '"' -f 4)

if [ -z "$url" ]; then
  echo "pvectl: no release asset found for ${os}_${arch}" >&2
  exit 1
fi

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT

curl -fsSL "$url" | tar -xz -C "$tmpdir" pvectl

sudo_cmd=""
if [ ! -w "$install_dir" ]; then
  sudo_cmd="sudo"
fi

$sudo_cmd install -m 755 "$tmpdir/pvectl" "$install_dir/pvectl"
echo "pvectl installed to $install_dir/pvectl"
