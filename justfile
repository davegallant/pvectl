# pvectl has no cgo dependencies; disable it so builds don't require a C
# compiler on systems where CGO_ENABLED defaults to 1 (e.g. no gcc in PATH).
export CGO_ENABLED := "0"

# List available recipes
default:
    @just --list

# Build the pvectl binary
build:
    go build -o pvectl ./cmd/pvectl

# Run pvectl directly from source
run *args:
    go run ./cmd/pvectl {{args}}

# Run the test suite
test:
    go test ./...

# Run go vet
vet:
    go vet ./...

# Format all Go source
fmt:
    gofmt -l -w .

# Tidy go.mod/go.sum
tidy:
    go mod tidy

# Install pvectl to $GOBIN (or $GOPATH/bin)
install:
    go install ./cmd/pvectl

# Remove build artifacts
clean:
    rm -f pvectl dist

# Run vet, lint, test, and build together
check: vet lint test build

# Run golangci-lint (needs golangci-lint on PATH)
lint:
    golangci-lint run

# Regenerate docs/cli/ from the actual cobra command tree
docs:
    go run ./tools/gendocs

# Record an asciinema demo of pvectl via scripts/demo.sh and convert it to a
# GIF for embedding in the README (requires asciinema and agg on PATH).
# Builds first so the recording reflects current source.
demo: build
    asciinema rec -c ./scripts/demo.sh pvectl-demo.cast --overwrite --window-size 100x24
    agg pvectl-demo.cast pvectl-demo.gif

# Tag and push a release using the CHANGELOG.md entry as the tag message,
# e.g. just release 0.1.0. Requires a clean working tree and a matching
# "## <version>" heading in CHANGELOG.md; pushes HEAD and the new tag to
# origin, which triggers the goreleaser GitHub Action.
release version:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -z "{{version}}" ]; then
      echo "version is required, e.g. just release 0.1.0"
      exit 1
    fi
    if [ -n "$(git status --porcelain)" ]; then
      echo "Working tree is not clean"
      exit 1
    fi
    notes="$(awk -v ver="## {{version}}" '$0==ver{f=1;next} /^## /{f=0} f' CHANGELOG.md | sed '/^$/d')"
    if [ -z "$notes" ]; then
      echo "No CHANGELOG.md entry found for version {{version}} (expected a '## {{version}}' heading)"
      exit 1
    fi
    git push origin HEAD
    git tag -a "v{{version}}" -m "$notes"
    git push origin "v{{version}}"
