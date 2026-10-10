# floe — local development tasks. https://just.systems

pkgs       := "./..."
cover_file := "coverage.out"
# Default version is the trimmed contents of VERSION; override with `VERSION=1.2.3 just build`.
version    := env_var_or_default("VERSION", trim(`cat VERSION 2>/dev/null || echo 0.0.0-dev`))
ldflags    := "-s -w -X github.com/sukekyo26/floe/internal/version.Version=" + version
targets    := "linux-amd64 linux-arm64 darwin-amd64 darwin-arm64"

# List available recipes
default:
    @just --list

# Install the pinned dev tools `just ci` needs into $(go env GOBIN) or GOPATH/bin.
# Versions must match .github/workflows; shellcheck comes from the OS package manager.
setup:
    #!/usr/bin/env bash
    set -euo pipefail
    bindir="$(go env GOBIN)"
    [ -n "${bindir}" ] || bindir="$(go env GOPATH | cut -d: -f1)/bin"
    mkdir -p "${bindir}"
    go install golang.org/x/vuln/cmd/govulncheck@v1.8.0
    go install mvdan.cc/sh/v3/cmd/shfmt@v3.10.0
    # The pinned install.sh SHA256-verifies the downloaded golangci-lint binary.
    curl -sSfL --proto '=https' --tlsv1.2 \
        https://raw.githubusercontent.com/golangci/golangci-lint/114493f9b3e7257d29e4130f2b4a4aadefbb6845/install.sh \
        | sh -s -- -b "${bindir}" v2.14.0
    command -v shellcheck >/dev/null 2>&1 \
        || echo >&2 "NOTE: shellcheck not found — install via 'apt-get install shellcheck' or 'brew install shellcheck'"
    echo "Done. Ensure ${bindir} is on your PATH, then run 'just ci'."

# Format Go source (gofumpt + goimports + gci via golangci-lint)
fmt:
    golangci-lint fmt {{pkgs}}

# Verify the tree is formatted
fmt-check:
    golangci-lint fmt --diff {{pkgs}}

# Run golangci-lint with the strict config
lint:
    golangci-lint run {{pkgs}}

# Run `go vet`
vet:
    go vet {{pkgs}}

# Run tests in shuffled order
test:
    go test -shuffle=on {{pkgs}}

# Run tests with the race detector (needs CGO and a C compiler)
test-race:
    CGO_ENABLED=1 go test -race -shuffle=on {{pkgs}}

# Fail when total coverage of internal/ is below MIN_COVERAGE (default 90)
cover-check:
    #!/usr/bin/env bash
    set -euo pipefail
    min_coverage="${MIN_COVERAGE:-90}"
    go test -shuffle=on -covermode=atomic -coverpkg=./internal/... -coverprofile={{cover_file}} {{pkgs}}
    total=$(go tool cover -func={{cover_file}} | awk '/^total:/ {gsub("%","",$3); print $3}')
    echo "Total coverage: ${total}% (threshold: ${min_coverage}%)"
    awk -v t="$total" -v m="$min_coverage" 'BEGIN { exit (t+0 >= m+0) ? 0 : 1 }' || {
        echo "::error::Coverage ${total}% is below threshold ${min_coverage}%"
        exit 1
    }

# Run govulncheck against the module
vuln:
    govulncheck {{pkgs}}

# Verify go.sum integrity
mod-verify:
    go mod verify

# Build bin/floe for the host
build:
    @mkdir -p bin
    CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags "{{ldflags}}" -o bin/floe ./cmd/floe

# Cross-compile bin/floe-<os>-<arch> for every release target
build-all:
    #!/usr/bin/env bash
    set -euo pipefail
    mkdir -p bin
    for target in {{targets}}; do
        CGO_ENABLED=0 GOOS="${target%-*}" GOARCH="${target#*-}" \
            go build -trimpath -buildvcs=false -ldflags "{{ldflags}}" -o "bin/floe-${target}" ./cmd/floe
    done

# Build every release binary and write bin/SHA256SUMS
release-assets: build-all
    #!/usr/bin/env bash
    set -euo pipefail
    cd bin
    sha256sum $(for t in {{targets}}; do printf 'floe-%s ' "$t"; done) > SHA256SUMS
    echo "wrote bin/SHA256SUMS"

# Run shellcheck on the shell scripts
shellcheck:
    shellcheck --severity=style install.sh

# Verify shell scripts are shfmt-clean
shfmt-check:
    shfmt -i 2 -ci -d install.sh

# Pre-push gate mirroring CI
ci: fmt-check vet lint test cover-check vuln mod-verify shellcheck shfmt-check
