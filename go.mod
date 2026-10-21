module github.com/sukekyo26/floe

go 1.27.3

require (
	github.com/chromedp/cdproto v0.157.9
	github.com/chromedp/chromedp v0.21.0
	github.com/expr-lang/expr v1.17.8
	github.com/goccy/go-yaml v1.19.2
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3
	github.com/spf13/cobra v1.10.2
	github.com/tdewolff/parse/v2 v2.8.19
	github.com/yuin/goldmark v1.8.6
	golang.org/x/net v0.61.0
	golang.org/x/sys v0.49.0
	golang.org/x/text v0.43.0
)

require (
	github.com/go-json-experiment/json v0.0.0-20260213210345-44df1a37e875 // indirect
	github.com/google/renameio/v2 v2.0.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/rogpeppe/go-internal v1.13.1 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	golang.org/x/mod v0.41.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/telemetry v0.0.0-20260924152758-ed294f943157 // indirect
	golang.org/x/term v0.47.0 // indirect
	golang.org/x/tools v0.51.0 // indirect
	golang.org/x/vuln v1.8.0 // indirect
	mvdan.cc/editorconfig v0.3.0 // indirect
	mvdan.cc/sh/v3 v3.10.0 // indirect
)

tool (
	golang.org/x/vuln/cmd/govulncheck
	mvdan.cc/sh/v3/cmd/shfmt
)
