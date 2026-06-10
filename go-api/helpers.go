package main

import (
	"io"
	"strconv"
	"strings"
)

// stringReader adapts a string to an io.Reader for request bodies.
func stringReader(s string) io.Reader { return strings.NewReader(s) }

// strconvQuote safely quotes a string for embedding in a JSON error body.
func strconvQuote(s string) string { return strconv.Quote(s) }

// truncate caps byte slices used in error messages so logs stay readable.
func truncate(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n]) + "…"
	}
	return string(b)
}
