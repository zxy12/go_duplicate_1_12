package main

import (
	"flag"
	"fmt"
	"os"
)

func _cmdenv() {
	path := flag.Bool("p", false, "emit updated PATH")
	plan9 := flag.Bool("9", false, "emit plan 9 syntax")
	windows := flag.Bool("w", false, "emit windows syntax")
	xflagparse(0)

	format := "%s=\"%s\"\n"
	switch {
	case *plan9:
		format = "%s='%s'\n"
	case *windows:
		format = "set %s=%s\r\n"
	}

	xprintf(format, "GOARCH", goarch)
	xprintf(format, "GOBIN", gobin)
	xprintf(format, "GOCACHE", os.Getenv("GOCACHE"))
	xprintf(format, "GODEBUG", os.Getenv("GODEBUG"))
	xprintf(format, "GOHOSTARCH", gohostarch)
	xprintf(format, "GOHOSTOS", gohostos)
	xprintf(format, "GOOS", goos)
	xprintf(format, "GOPROXY", os.Getenv("GOPROXY"))
	xprintf(format, "GOROOT", goroot)
	xprintf(format, "GOTMPDIR", os.Getenv("GOTMPDIR"))
	xprintf(format, "GOTOOLDIR", tooldir)
	if goarch == "arm" {
		xprintf(format, "GOARM", goarm)
	}
	if goarch == "386" {
		xprintf(format, "GO386", go386)
	}
	if goarch == "mips" || goarch == "mipsle" {
		xprintf(format, "GOMIPS", gomips)
	}
	if goarch == "mips64" || goarch == "mips64le" {
		xprintf(format, "GOMIPS64", gomips64)
	}

	if *path {
		sep := ":"
		if gohostos == "windows" {
			sep = ";"
		}
		xprintf(format, "PATH", fmt.Sprintf("%s%s%s", gobin, sep, os.Getenv("PATH")))
	}
}
