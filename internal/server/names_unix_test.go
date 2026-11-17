//go:build unix

package server_test

// osBadImageNames are image names Windows reads otherwise; Unix takes them as they are.
var osBadImageNames []string //nolint:gochecknoglobals // a test's table, empty here
