#!/bin/sh
# The single-quoted fixture scripts must preserve variables for the mock
# commands to expand at execution time, not during fixture generation.
# shellcheck disable=SC2016
set -eu

repo_dir=$(CDPATH='' cd "$(dirname "$0")/.." && pwd)
test_dir=$(mktemp -d)
trap 'rm -rf "$test_dir"' EXIT
mkdir -p "$test_dir/bin" "$test_dir/fixture"

printf '#!/bin/sh\ncase "$1" in -s) printf "%%s\\n" "${MOCK_OS:-Linux}" ;; -m) printf "%%s\\n" "${MOCK_ARCH:-x86_64}" ;; esac\n' > "$test_dir/bin/uname"
chmod +x "$test_dir/bin/uname"

printf '%s\n' '#!/bin/sh' \
  'set -eu' \
  'out=""' \
  'url=""' \
  'while [ "$#" -gt 0 ]; do' \
  '  case "$1" in -o) out=$2; shift 2 ;; -*) shift ;; *) url=$1; shift ;; esac' \
  'done' \
  'printf "%s\n" "$url" >> "$FIXTURE_DIR/requests"' \
  'case "$url" in' \
  '  */releases/latest) source_file="$FIXTURE_DIR/release.json" ;;' \
  '  */releases/tags/v1.2.3) source_file="$FIXTURE_DIR/release.json" ;;' \
  '  */checksums.txt) source_file="$FIXTURE_DIR/checksums.txt" ;;' \
  '  */pvectl_1.2.3_linux_amd64.tar.gz) source_file="$FIXTURE_DIR/archive.tar.gz" ;;' \
  '  *) echo "unexpected URL: $url" >&2; exit 1 ;;' \
  'esac' \
  'if [ -n "$out" ]; then cp "$source_file" "$out"; else sed -n p "$source_file"; fi' \
  > "$test_dir/bin/curl"
chmod +x "$test_dir/bin/curl"

printf 'pvectl test binary\n' > "$test_dir/fixture/pvectl"
tar -czf "$test_dir/fixture/archive.tar.gz" -C "$test_dir/fixture" pvectl
if command -v sha256sum >/dev/null 2>&1; then
  digest=$(sha256sum "$test_dir/fixture/archive.tar.gz" | cut -d ' ' -f 1)
else
  digest=$(shasum -a 256 "$test_dir/fixture/archive.tar.gz" | cut -d ' ' -f 1)
fi
printf '%s  %s\n' "$digest" pvectl_1.2.3_linux_amd64.tar.gz > "$test_dir/fixture/checksums.txt"
printf '%s\n' '{"tag_name":"v1.2.3","assets":[' \
  '{"browser_download_url":"https://example.invalid/pvectl_1.2.3_linux_amd64.tar.gz"},' \
  '{"browser_download_url":"https://example.invalid/checksums.txt"}]}' \
  > "$test_dir/fixture/release.json"

fixture_env() {
  PATH="$test_dir/bin:$PATH" FIXTURE_DIR="$test_dir/fixture" INSTALL_DIR="$test_dir/output" PVECTL_VERSION=v1.2.3 "$@"
}

fixture_env sh "$repo_dir/scripts/install.sh" > "$test_dir/success.out" 2> "$test_dir/success.err"
test -f "$test_dir/output/pvectl"
test "$(sed -n p "$test_dir/output/pvectl")" = 'pvectl test binary'
grep -q '/releases/tags/v1.2.3' "$test_dir/fixture/requests"
if grep -q '/releases/latest' "$test_dir/fixture/requests"; then
  echo 'pinned install queried latest release' >&2
  exit 1
fi

rm -f "$test_dir/output/pvectl"
printf '%064d  %s\n' 0 pvectl_1.2.3_linux_amd64.tar.gz > "$test_dir/fixture/checksums.txt"
if fixture_env sh "$repo_dir/scripts/install.sh" > "$test_dir/bad.out" 2> "$test_dir/bad.err"; then
  echo 'checksum mismatch unexpectedly succeeded' >&2
  exit 1
fi
test ! -e "$test_dir/output/pvectl"
grep -qi 'checksum' "$test_dir/bad.err"

requests_before=$(wc -l < "$test_dir/fixture/requests")
if MOCK_OS=FreeBSD fixture_env sh "$repo_dir/scripts/install.sh" > "$test_dir/os.out" 2> "$test_dir/os.err"; then
  echo 'unsupported OS unexpectedly succeeded' >&2
  exit 1
fi
grep -qi 'unsupported.*os' "$test_dir/os.err"
test "$(wc -l < "$test_dir/fixture/requests")" -eq "$requests_before"

if MOCK_ARCH=riscv64 fixture_env sh "$repo_dir/scripts/install.sh" > "$test_dir/arch.out" 2> "$test_dir/arch.err"; then
  echo 'unsupported architecture unexpectedly succeeded' >&2
  exit 1
fi
grep -qi 'unsupported architecture' "$test_dir/arch.err"

if PATH="$test_dir/bin:$PATH" FIXTURE_DIR="$test_dir/fixture" INSTALL_DIR="$test_dir/output" PVECTL_VERSION='' \
  sh "$repo_dir/scripts/install.sh" > "$test_dir/version.out" 2> "$test_dir/version.err"; then
  echo 'empty pinned version unexpectedly succeeded' >&2
  exit 1
fi
grep -q 'invalid PVECTL_VERSION' "$test_dir/version.err"

echo 'installer tests passed'
