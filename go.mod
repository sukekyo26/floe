module github.com/sukekyo26/floe

go 1.27.3

require (
	github.com/chromedp/cdproto v0.158.5
	github.com/chromedp/chromedp v0.21.0
	github.com/expr-lang/expr v1.17.8
	github.com/goccy/go-yaml v1.19.2
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3
	github.com/spf13/cobra v1.10.2
	github.com/tdewolff/parse/v2 v2.8.20
	github.com/yuin/goldmark v1.8.6
	golang.org/x/net v0.61.0
	golang.org/x/sys v0.49.0
	golang.org/x/text v0.43.0
)

require (
	github.com/go-json-experiment/json v0.0.0-20260213210345-44df1a37e875 // indirect
	github.com/google/renameio/v2 v2.0.2 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/rogpeppe/go-internal v1.16.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	golang.org/x/mod v0.42.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/telemetry v0.0.0-20261020154912-7bf0a11256d5 // indirect
	golang.org/x/term v0.47.0 // indirect
	golang.org/x/tools v0.52.0 // indirect
	golang.org/x/vuln v1.9.0 // indirect
	mvdan.cc/editorconfig v0.3.0 // indirect
	mvdan.cc/sh/v3 v3.14.1 // indirect
)

tool (
	golang.org/x/vuln/cmd/govulncheck
	mvdan.cc/sh/v3/cmd/shfmt
)
