module floe

go 1.27.4

require (
	github.com/chromedp/cdproto v0.160.10
	github.com/chromedp/chromedp v0.24.0
	github.com/expr-lang/expr v1.18.1
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3
	github.com/sergi/go-diff v1.4.5
	github.com/spf13/cobra v1.10.2
	github.com/spf13/pflag v1.1.0
	github.com/tats-u/goldmark-cjk-friendly/v2 v2.0.3
	github.com/tdewolff/parse/v2 v2.25.0
	github.com/yuin/goldmark v1.8.7
	go.yaml.in/yaml/v4 v4.0.0
	golang.org/x/net v0.63.0
	golang.org/x/sys v0.50.0
	golang.org/x/text v0.44.0
)

require (
	github.com/go-json-experiment/json v0.0.0-20260213210345-44df1a37e875 // indirect
	github.com/google/renameio/v2 v2.0.2 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/rogpeppe/go-internal v1.16.0 // indirect
	golang.org/x/mod v0.43.0 // indirect
	golang.org/x/sync v0.25.0 // indirect
	golang.org/x/telemetry v0.0.0-20261116191925-e77a43154acb // indirect
	golang.org/x/term v0.48.0 // indirect
	golang.org/x/tools v0.53.0 // indirect
	golang.org/x/vuln v1.10.0 // indirect
	mvdan.cc/editorconfig v0.3.0 // indirect
	mvdan.cc/sh/v3 v3.14.1 // indirect
)

tool (
	golang.org/x/vuln/cmd/govulncheck
	mvdan.cc/sh/v3/cmd/shfmt
)

ignore ./internal/ui/web/node_modules
