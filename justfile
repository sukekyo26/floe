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

# Install golangci-lint into $(go env GOBIN) or GOPATH/bin. CI runs this recipe, so the
# version below is the only pin. govulncheck and shfmt are pinned as tools in go.mod;
# shellcheck comes from the OS package manager.
setup:
    #!/usr/bin/env bash
    set -euo pipefail
    bindir="$(go env GOBIN)"
    [ -n "${bindir}" ] || bindir="$(go env GOPATH | cut -d: -f1)/bin"
    mkdir -p "${bindir}"
    # The pinned install.sh SHA256-verifies the downloaded golangci-lint binary.
    curl -sSfL --proto '=https' --tlsv1.2 \
        https://raw.githubusercontent.com/golangci/golangci-lint/114493f9b3e7257d29e4130f2b4a4aadefbb6845/install.sh \
        | sh -s -- -b "${bindir}" v2.15.0
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

# Write the golden files of internal/corpus anew from what floe prints and draws now; read the change in git diff
golden:
    go test ./internal/corpus -run TestCorpus -update

# Fail when total coverage of internal/ is below MIN_COVERAGE (default 90)
cover-check:
    #!/usr/bin/env bash
    set -euo pipefail
    min_coverage="${MIN_COVERAGE:-90}"
    # Without internal/corpus: what its samples run through cli.Run would count for the unit tests of every package.
    # A plain substitution, which fails the recipe as pipefail tells (bash 3.2, on macOS, has no mapfile).
    packages=$(go list {{pkgs}} | grep -v '/internal/corpus$') || {
        echo "cover-check: go list found no package to test but internal/corpus; run go list {{pkgs}}" >&2
        exit 1
    }
    # shellcheck disable=SC2086 # one package to a word; an import path has no space
    go test -shuffle=on -covermode=atomic -coverpkg=./internal/... -coverprofile={{cover_file}} $packages
    total=$(go tool cover -func={{cover_file}} | awk '/^total:/ {gsub("%","",$3); print $3}')
    echo "Total coverage: ${total}% (threshold: ${min_coverage}%)"
    awk -v t="$total" -v m="$min_coverage" 'BEGIN { exit (t+0 >= m+0) ? 0 : 1 }' || {
        echo "::error::Coverage ${total}% is below threshold ${min_coverage}%"
        exit 1
    }

# Run govulncheck against the module
vuln:
    go tool govulncheck {{pkgs}}

# Verify go.sum integrity
mod-verify:
    go mod verify

# Verify go.mod and go.sum are tidy (direct and indirect requirements, no unused modules)
mod-tidy-check:
    go mod tidy -diff

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
    # macOS ships shasum instead of sha256sum; both print "<hash>  <file>".
    if command -v sha256sum >/dev/null 2>&1; then sum=(sha256sum)
    elif command -v shasum >/dev/null 2>&1; then sum=(shasum -a 256)
    else echo "release-assets: install sha256sum (coreutils) or shasum (perl)" >&2; exit 1
    fi
    "${sum[@]}" $(for t in {{targets}}; do printf 'floe-%s ' "$t"; done) > SHA256SUMS
    echo "wrote bin/SHA256SUMS"

# Run shellcheck on the shell scripts
shellcheck:
    shellcheck --severity=style install.sh

# Verify shell scripts are shfmt-clean
shfmt-check:
    go tool shfmt -i 2 -ci -d install.sh

# Pre-push gate mirroring CI
ci: fmt-check vet lint test cover-check vuln mod-verify mod-tidy-check shellcheck shfmt-check
