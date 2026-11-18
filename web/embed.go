// Package web holds the page the server serves: plain HTML, CSS and JavaScript, no build step.
package web

import "embed"

// Files are the page's files, embedded in the binary.
//
//go:embed index.html icon.svg css js locales
var Files embed.FS
