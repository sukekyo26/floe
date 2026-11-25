# floe — local development tasks. https://just.systems

pkgs       := "./..."
cover_file := "coverage.out"
# The packages but the browser tests, as a command that lists them: run in the recipe, not each time just reads this file.
core_pkgs  := "go list " + pkgs + " | grep -v /internal/browsertest$"
# The browser tests run in parts, a CI job each (browser-parts): looks is TestLookRule, and the other tests go by name in
# turn to rest-1 … rest-<browser_rest_jobs>. A part that takes over 8/10 of browser_timeout (seconds) gets a warning on CI.
# LIMIT: how even the rest parts are is luck, and is not checked; the warning tells it. Then raise browser_rest_jobs by one;
# if the heaviest part stays over, by one more; if still, split by each test's measured time instead. TestLookRule is one
# test of 27 t.Run, which -run 'TestLookRule/…' can split when looks gets the warning. The time measured starts as -timeout
# does, after the -race build, which go test -list has done.
browser_rest_jobs := "2"
browser_timeout   := "600"
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

# LIMIT: the files of //go:build race are not read (no -race): six constants of 5 lines; add a -race run when one holds more.
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
    for _ in 1 2 3 4 5; do
        for os in {{fix_oses}}; do GOOS=$os go fix {{fix_flags}} {{pkgs}}; done
        golangci-lint fmt {{pkgs}}
        left=
        for os in {{fix_oses}}; do
            GOOS=$os go fix {{fix_flags}} -diff {{pkgs}} >/dev/null || left=1
        done
        if [ -z "$left" ]; then exit 0; fi
    done
    echo "fix: go fix still changes the code after 5 runs: read git diff" >&2
    exit 1

# Fail when go fix would change the code, for any GOOS
fix-check:
    #!/usr/bin/env bash
    set -euo pipefail
    for os in {{fix_oses}}; do
        GOOS=$os go fix {{fix_flags}} -diff {{pkgs}} || {
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

# The parts of the browser tests, CI's matrix of browser jobs, as JSON: looks, then rest-1 … rest-<browser_rest_jobs>
browser-parts:
    #!/usr/bin/env bash
    set -euo pipefail
    jobs="{{browser_rest_jobs}}"
    if [[ ! $jobs =~ ^[1-9][0-9]*$ ]]; then
        echo "browser-parts: browser_rest_jobs is \"$jobs\"; make it a whole number from 1" >&2; exit 1
    fi
    parts='"looks"'
    for ((k = 1; k <= jobs; k++)); do parts+=",\"rest-$k\""; done
    echo "[$parts]"

# LIMIT: without Chrome every test skips and it passes; it runs what CI does on a machine with Chrome alone.
# Run one part of the browser tests with the race detector, as CI's job of it does: looks, or rest-1 … (just browser-parts)
test-browser part:
    #!/usr/bin/env bash
    set -euo pipefail
    part="{{part}}"
    parts=$("{{just_executable()}}" --justfile "{{justfile()}}" browser_rest_jobs="{{browser_rest_jobs}}" browser-parts)
    if [[ $parts != *"\"$part\""* ]]; then
        echo "test-browser: no part $part; use one of $parts" >&2; exit 1
    fi
    # -race as the run below, so that one test binary is built; a build that fails stops here, not as names missing.
    listed=$(CGO_ENABLED=1 go test -race -list . ./internal/browsertest)
    mapfile -t names < <(grep -v -e '^ok ' -e '^$' <<<"$listed" | LC_ALL=C sort)
    if [ ${#names[@]} -eq 0 ]; then
        echo "test-browser: go test -list tells no test of internal/browsertest" >&2; exit 1
    fi
    looks=
    for name in "${names[@]}"; do
        # A part runs its tests by -run, which an example, fuzz or benchmark would be in no part of.
        if [[ $name != Test* ]]; then
            echo "test-browser: $name is not a Test…, which no part runs; make it a test, or give it a part here" >&2; exit 1
        fi
        if [ "$name" = TestLookRule ]; then looks=1; fi
    done
    if [ -z "$looks" ]; then
        echo "test-browser: no TestLookRule in internal/browsertest, which is the part looks; change the parts here" >&2; exit 1
    fi
    pick=()
    if [ "$part" = looks ]; then
        pick=(TestLookRule)
    else
        k=${part#rest-} i=0
        for name in "${names[@]}"; do
            if [ "$name" = TestLookRule ]; then continue; fi
            if [ $((i % {{browser_rest_jobs}} + 1)) -eq "$k" ]; then pick+=("$name"); fi
            i=$((i + 1))
        done
        if [ ${#pick[@]} -eq 0 ]; then
            echo "test-browser: part $part has no test; lower browser_rest_jobs" >&2; exit 1
        fi
    fi
    run=$(IFS='|'; echo "${pick[*]}")
    # Past -timeout the test binary stops itself and prints the tests running; the job stops later, with nothing printed.
    start=$SECONDS code=0
    CGO_ENABLED=1 go test -race -shuffle=on -timeout {{browser_timeout}}s -run "^($run)\$" ./internal/browsertest || code=$?
    took=$((SECONDS - start)) over=$(({{browser_timeout}} * 8 / 10))
    # Told on a failure too: a run that timed out is the one most over.
    if [ "$took" -gt "$over" ]; then
        if [ "$part" = looks ]; then
            fix="split TestLookRule by its rules, with -run 'TestLookRule/…'"
        else
            fix="raise browser_rest_jobs in the justfile by one; if the heaviest part stays over, by one more; if still, split by each test's time"
        fi
        echo "::warning title=Browser ($part) took ${took}s::over ${over}s, 8/10 of -timeout {{browser_timeout}}s: $fix"
    fi
    exit "$code"

# LIMIT: the tests are named here, corpus first, as pagediff's golden is read from corpus's golden page: a new test of golden files is missed unless it is added here
# Write every golden file anew from what floe prints and draws now; read the change in git diff
golden:
    go test ./internal/corpus -run TestCorpus -update
    go test ./internal/diagram -run 'TestExamples|TestFaceOfGolden' -update
    go test ./internal/pagediff -run TestReadEveryPart -update
    go test ./internal/store -run TestOwnerAnswers -update

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

web_dir := "internal/ui/web"

# Install the management page's packages as pnpm-lock.yaml pins them (Node and pnpm are needed only for the web recipes)
web-install:
    cd {{web_dir}} && pnpm install --frozen-lockfile

# Format, type, lint and unit tests of the management page
web-check: web-install
    cd {{web_dir}} && pnpm run check

# Build the management page into dist/, which is committed and embedded by Go
web-build: web-install
    cd {{web_dir}} && pnpm run build

# The build gives the same bytes twice, and they are the committed dist/; dist/ is left as it is. The copy of Tailwind's
# license, which no build output holds, is the installed one's.
web-build-check: web-install
    #!/usr/bin/env bash
    set -euo pipefail
    cd {{web_dir}}
    # Locally, a dist/ changed by hand or not committed; a fresh checkout in CI always passes this
    if [ -n "$(git status --porcelain --untracked-files=all -- dist)" ]; then
        git status --short --untracked-files=all -- dist >&2
        echo "error: dist differs from what is committed: commit it, or restore it with git checkout -- dist" >&2
        exit 1
    fi
    tmp=$(mktemp -d)
    trap 'rm -rf "$tmp"' EXIT
    pnpm run build --outDir "$tmp/one" >/dev/null
    pnpm run build --outDir "$tmp/two" >/dev/null
    diff -r "$tmp/one" "$tmp/two" >/dev/null \
        || { echo "error: two builds of the same source differ: the build is not reproducible" >&2; exit 1; }
    diff -rq "$tmp/one" dist >&2 \
        || { echo "error: dist is not the build of src: run just web-build and commit dist" >&2; exit 1; }
    cmp -s node_modules/tailwindcss/LICENSE tailwindcss.LICENSE \
        || { echo "error: tailwindcss.LICENSE is not the installed tailwindcss's license: copy node_modules/tailwindcss/LICENSE to it and run just licenses" >&2; exit 1; }

# LIMIT: test runs every package without the race detector; a race is told by CI alone (test-race-core and the
# browser jobs, all with -race).
# Pre-push gate of CI's checks
ci: fmt-check vet lint fix-check test cover-check vuln mod-verify mod-tidy-check shellcheck shfmt-check

# Serve the gallery of the management page's shared components on 127.0.0.1:<port>, in lang (ja or en), for a person to
# try them: go test writes its boot values, and Vite's dev server serves it. It is never built into dist
web-gallery port lang="ja": web-install
    go test ./internal/ui -run '^TestGalleryBoot$' -count=1 -gallery={{lang}}
    cd {{web_dir}} && pnpm exec vite --config vite.config.ts gallery --host 127.0.0.1 --port {{port}} --strictPort
