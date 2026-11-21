# floe — local development tasks. https://just.systems

pkgs       := "./..."
cover_file := "coverage.out"
# The packages but the browser tests, as a command that lists them: run in the recipe, not each time just reads this file.
core_pkgs  := "go list " + pkgs + " | grep -v /internal/browsertest$"
# The browser tests CI runs on one machine (test-browser looks); the rest run on another. Chosen by adding the heaviest
# first (go test -race -json, each test's time) while the sum nears half of all: choose again so when a test is added.
# LIMIT: how even the two are is not checked; CI's two jobs' times show it. When one job passes 8 minutes (its timeout is
# 15), split the tests into three jobs, and change the required checks of the develop ruleset with them.
browser_looks := "TestLookRule|TestUITaskChildren|TestUIVersionGone|TestUITaskPage|TestUITaskArchive|TestUITaskParent|TestUIDiffNotice"
# Default version is the trimmed contents of VERSION; override with `VERSION=1.2.3 just build`.
version    := env_var_or_default("VERSION", trim(`cat VERSION 2>/dev/null || echo 0.0.0-dev`))
ldflags    := "-s -w -X floe/internal/version.Version=" + version
targets    := "linux-amd64 linux-arm64 darwin-amd64 darwin-arm64 windows-amd64 windows-arm64"
# The file of each target in bin/: Windows runs a program only under a name that ends in .exe.
assets     := replace_regex(replace_regex(targets, '\S+', 'floe-$0'), 'floe-windows-\S+', '$0.exe')

# List available recipes
default:
    @just --list

# The only pin of golangci-lint, raised by hand; CI also keys its cache of the binary on it.
golangci_version := "v2.15.0"

# Install golangci-lint into $(go env GOBIN) or GOPATH/bin. CI runs this recipe on a cache miss.
# govulncheck and shfmt are pinned as tools in go.mod; shellcheck comes from the OS package manager.
setup:
    #!/usr/bin/env bash
    set -euo pipefail
    bindir="$(go env GOBIN)"
    [ -n "${bindir}" ] || bindir="$(go env GOPATH | cut -d: -f1)/bin"
    mkdir -p "${bindir}"
    # The pinned install.sh SHA256-verifies the downloaded golangci-lint binary.
    curl -sSfL --proto '=https' --tlsv1.2 \
        https://raw.githubusercontent.com/golangci/golangci-lint/114493f9b3e7257d29e4130f2b4a4aadefbb6845/install.sh \
        | sh -s -- -b "${bindir}" {{golangci_version}}
    command -v shellcheck >/dev/null 2>&1 \
        || echo >&2 "NOTE: shellcheck not found — install via 'apt-get install shellcheck' or 'brew install shellcheck'"
    echo "Done. Ensure ${bindir} is on your PATH, then run 'just ci'."

# Format Go source (gofumpt + goimports + gci via golangci-lint)
fmt:
    golangci-lint fmt {{pkgs}}

# Verify the tree is formatted
fmt-check:
    golangci-lint fmt --diff {{pkgs}}

# LIMIT: internal/browsertest is left out (core_pkgs) while its tests are rewritten for the React management page; use pkgs once that is done.
# LIMIT: the files of //go:build race are not read (no -race): four constants of 5 lines; add a -race run when one holds more.
# LIMIT: core_pkgs lists the host GOOS's packages: a package of one OS alone fails the other GOOS's runs; list them per GOOS then.
# LIMIT: just lint type-checks linux and windows only: when fix-check fails for darwin and just fix does not help, it is a type error of darwin's files.
# LIMIT: the go fix of the Go on PATH runs: one newer than go.mod's may know more fixers and fail fix-check on a clean tree.
# go fix reads one GOOS's files a run: the three floe runs on.
fix_oses  := "linux darwin windows"
# omitzero changes what encoding/json writes (a zero struct left out), so it is not applied.
fix_flags := "-omitzero=false"

# Rewrite Go to the newer forms go fix knows, for each GOOS, and format, until go fix changes nothing more
fix:
    #!/usr/bin/env bash
    set -euo pipefail
    fix_pkgs=$({{core_pkgs}})
    for _ in 1 2 3 4 5; do
        for os in {{fix_oses}}; do GOOS=$os go fix {{fix_flags}} $fix_pkgs; done
        golangci-lint fmt {{pkgs}}
        left=
        for os in {{fix_oses}}; do
            GOOS=$os go fix {{fix_flags}} -diff $fix_pkgs >/dev/null || left=1
        done
        if [ -z "$left" ]; then exit 0; fi
    done
    echo "fix: go fix still changes the code after 5 runs: read git diff" >&2
    exit 1

# Fail when go fix would change the code, for any GOOS
fix-check:
    #!/usr/bin/env bash
    set -euo pipefail
    fix_pkgs=$({{core_pkgs}})
    for os in {{fix_oses}}; do
        GOOS=$os go fix {{fix_flags}} -diff $fix_pkgs || {
            echo "fix-check: go fix failed or would change the code for GOOS=$os: run 'just fix' for a diff above; with no diff, fix the compiler's errors above" >&2
            exit 1
        }
    done

# Run golangci-lint with the strict config, on Linux and on Windows (whose files only build there), and vet Windows
# on arm64, which no CI machine runs
lint:
    golangci-lint run {{pkgs}}
    GOOS=windows golangci-lint run {{pkgs}}
    GOOS=windows GOARCH=arm64 go vet {{pkgs}}

# Run `go vet`
vet:
    go vet {{pkgs}}

# Run tests in shuffled order
test:
    go test -shuffle=on {{pkgs}}

# Run tests with the race detector (needs CGO and a C compiler); the heaviest random tests of internal/diagram run a fifth of their graphs
test-race:
    CGO_ENABLED=1 go test -race -shuffle=on {{pkgs}}

# -timeout stays under CI's 10 minutes, so that a slow or stuck run names the tests still running, not just a cancelled job.
# Run tests with the race detector but the browser tests, as CI's test job does; the heaviest random tests of internal/diagram run a fifth of their graphs
test-race-core:
    CGO_ENABLED=1 go test -race -shuffle=on -timeout 9m $({{core_pkgs}})

# LIMIT: without Chrome every test skips and it passes; it runs what CI does on a machine with Chrome alone.
# Run one part of the browser tests with the race detector, as CI's job of it does: looks (browser_looks) or rest
test-browser part:
    #!/usr/bin/env bash
    set -euo pipefail
    case "{{part}}" in
    looks) pick=-run ;;
    rest) pick=-skip ;;
    *) echo "test-browser: no part {{part}}; use looks or rest" >&2; exit 1 ;;
    esac
    # -race as the run below, so that one test binary is built; a build that fails stops here, not as names missing.
    listed=$(CGO_ENABLED=1 go test -race -list . ./internal/browsertest)
    declare -A have=() seen=()
    for name in $(grep '^Test' <<<"$listed" || true); do have[$name]=1; done
    if [ ${#have[@]} -eq 0 ]; then
        echo "test-browser: go test -list tells no test of internal/browsertest" >&2; exit 1
    fi
    looks="{{browser_looks}}"
    # read drops an empty name at the end, which the first test tells.
    if [[ -z $looks || $looks == *'|' ]]; then
        echo "test-browser: browser_looks has an empty name" >&2; exit 1
    fi
    IFS='|' read -ra names <<<"$looks"
    for name in "${names[@]}"; do
        if [ -z "$name" ]; then
            echo "test-browser: browser_looks has an empty name" >&2; exit 1
        elif [ -n "${seen[$name]:-}" ]; then
            echo "test-browser: $name is twice in browser_looks" >&2; exit 1
        elif [ -z "${have[$name]:-}" ]; then
            echo "test-browser: $name is in browser_looks but no test of internal/browsertest; remove it" >&2; exit 1
        fi
        seen[$name]=1
    done
    # The job stops at 15 minutes with nothing printed; past 10 the test binary stops itself and prints the tests running.
    CGO_ENABLED=1 go test -race -shuffle=on -timeout 10m "$pick" '^({{browser_looks}})$' ./internal/browsertest

# LIMIT: the tests are named here, corpus first, as pagediff's golden is read from corpus's golden page: a new test of golden files is missed unless it is added here
# Write every golden file anew from what floe prints and draws now; read the change in git diff
golden:
    go test ./internal/corpus -run TestCorpus -update
    go test ./internal/diagram -run 'TestExamples|TestFaceOfGolden' -update
    go test ./internal/pagediff -run TestReadEveryPart -update

# Write internal/licenses/NOTICES.txt anew, which floe licenses prints, after a module is added, removed or upgraded
licenses:
    go test ./internal/licenses -run '^TestNotices$' -update

# Fail when total coverage of internal/ is below MIN_COVERAGE (default 90)
cover-check:
    #!/usr/bin/env bash
    set -euo pipefail
    min_coverage="${MIN_COVERAGE:-90}"
    # The browser tests are left out: without Chrome (skipped) they would lower the total, so that it would not be CI's.
    # set, as it runs without -race (which makes go count atomic): only whether a line ran counts, and set is the fastest
    # -timeout stays under CI's 10 minutes, as in test-race-core; this job runs all of the random graphs, so reproduce a failure here without -race.
    go test -shuffle=on -timeout 9m -covermode=set -coverpkg=./internal/... -coverprofile={{cover_file}} $({{core_pkgs}})
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
    for asset in {{assets}}; do
        target="${asset#floe-}"
        target="${target%.exe}"
        CGO_ENABLED=0 GOOS="${target%-*}" GOARCH="${target#*-}" \
            go build -trimpath -buildvcs=false -ldflags "{{ldflags}}" -o "bin/${asset}" ./cmd/floe
    done

# Fail when a release binary holds the owner's name: it is in the module path, so in every symbol, and in `go version -m`.
check-no-repo-name: build-all
    #!/usr/bin/env bash
    set -euo pipefail
    for asset in {{assets}}; do
        if grep -aq sukekyo26 "bin/${asset}"; then
            echo "check-no-repo-name: bin/${asset} holds 'sukekyo26': see where with 'go version -m bin/${asset}' and 'strings bin/${asset} | grep sukekyo26'; the module in go.mod stays 'floe'" >&2
            exit 1
        fi
    done

# Build every release binary and write bin/SHA256SUMS
release-assets: check-no-repo-name
    #!/usr/bin/env bash
    set -euo pipefail
    cd bin
    # macOS ships shasum instead of sha256sum; both print "<hash>  <file>".
    if command -v sha256sum >/dev/null 2>&1; then sum=(sha256sum)
    elif command -v shasum >/dev/null 2>&1; then sum=(shasum -a 256)
    else echo "release-assets: install sha256sum (coreutils) or shasum (perl)" >&2; exit 1
    fi
    "${sum[@]}" {{assets}} > SHA256SUMS
    echo "wrote bin/SHA256SUMS"

# List the files release-assets makes, a line each: what release.yml attests and publishes
release-files:
    #!/usr/bin/env bash
    set -euo pipefail
    for asset in {{assets}}; do echo "bin/${asset}"; done
    echo bin/SHA256SUMS

# Run shellcheck on the shell scripts
shellcheck:
    shellcheck --severity=style install.sh

# Verify shell scripts are shfmt-clean
shfmt-check:
    go tool shfmt -i 2 -ci -d install.sh

# LIMIT: test runs every package without the race detector; a race is told by CI alone (test-race-core and the two
# browser jobs, all with -race).
# Pre-push gate of CI's checks
ci: fmt-check vet lint fix-check test cover-check vuln mod-verify mod-tidy-check shellcheck shfmt-check
