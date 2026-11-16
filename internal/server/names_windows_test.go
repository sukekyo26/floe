package server_test

// osBadImageNames are image names Windows reads otherwise: a \ as a separator, a stream after :, a device.
var osBadImageNames = []string{`sub\b.png`, "a.png:x", "NUL"} //nolint:gochecknoglobals // a test's table
