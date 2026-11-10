// Package adr holds the architecture decision records of orbithek.
//
// Its tests keep the records consistent: file names, headings, front
// matter and the status rules. Its one piece of code reads the records'
// headings and front matter and makes the index in README.md from them;
// a test fails when the committed index is not the generated one. They run
// as part of `go test ./...`.
package adr

//go:generate go run ./cmd/adrindex
