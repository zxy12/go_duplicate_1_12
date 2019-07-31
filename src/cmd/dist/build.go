// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

/*
import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)
*/

import (
//"os"
//"path/filepath"
)

// Initialization for any invocation.

// The usual variables.
var (
	goarch           string
	gobin            string
	gohostarch       string
	gohostos         string
	goos             string
	goarm            string
	go386            string
	gomips           string
	gomips64         string
	goroot           string
	goroot_final     string
	goextlinkenabled string
	gogcflags        string // For running built compiler
	goldflags        string
	workdir          string
	tooldir          string
	oldgoos          string
	oldgoarch        string
	exe              string
	defaultcc        map[string]string
	defaultcxx       map[string]string
	defaultcflags    string
	defaultldflags   string
	defaultpkgconfig string

	rebuildall   bool
	defaultclang bool

	vflag int // verbosity
)

// The known architectures.
var okgoarch = []string{
	"386",
	"amd64",
	"amd64p32",
	"arm",
	"arm64",
	"mips",
	"mipsle",
	"mips64",
	"mips64le",
	"ppc64",
	"ppc64le",
	"riscv64",
	"s390x",
	"sparc64",
	"wasm",
}

// The known operating systems.
var okgoos = []string{
	"darwin",
	"dragonfly",
	"js",
	"linux",
	"android",
	"solaris",
	"freebsd",
	"nacl",
	"netbsd",
	"openbsd",
	"plan9",
	"windows",
	"aix",
}

// cleanlist is a list of packages with generated files and commands.
var cleanlist = []string{
	"runtime/internal/sys",
	"cmd/cgo",
	"cmd/go/internal/cfg",
	"go/build",
}

var runtimegen = []string{
	"zaexperiment.h",
	"zversion.go",
}

// gentab records how to generate some trivial files.
var gentab = []struct {
	nameprefix string
	gen        func(string, string)
}{
	{"zdefaultcc.go", mkzdefaultcc},
	{"zosarch.go", mkzosarch},
	{"zversion.go", mkzversion},
	{"zcgo.go", mkzcgo},

	// not generated anymore, but delete the file if we see it
	{"enam.c", nil},
	{"anames5.c", nil},
	{"anames6.c", nil},
	{"anames8.c", nil},
	{"anames9.c", nil},
}

// Cannot use go/build directly because cmd/dist for a new release
// builds against an old release's go/build, which may be out of sync.
// To reduce duplication, we generate the list for go/build from this.
//
// We list all supported platforms in this list, so that this is the
// single point of truth for supported platforms. This list is used
// by 'go tool dist list'.
var cgoEnabled = map[string]bool{
	"aix/ppc64":       false,
	"darwin/386":      true,
	"darwin/amd64":    true,
	"darwin/arm":      true,
	"darwin/arm64":    true,
	"dragonfly/amd64": true,
	"freebsd/386":     true,
	"freebsd/amd64":   true,
	"freebsd/arm":     false,
	"linux/386":       true,
	"linux/amd64":     true,
	"linux/arm":       true,
	"linux/arm64":     true,
	"linux/ppc64":     false,
	"linux/ppc64le":   true,
	"linux/mips":      true,
	"linux/mipsle":    true,
	"linux/mips64":    true,
	"linux/mips64le":  true,
	"linux/riscv64":   true,
	"linux/s390x":     true,
	"linux/sparc64":   true,
	"android/386":     true,
	"android/amd64":   true,
	"android/arm":     true,
	"android/arm64":   true,
	"js/wasm":         false,
	"nacl/386":        false,
	"nacl/amd64p32":   false,
	"nacl/arm":        false,
	"netbsd/386":      true,
	"netbsd/amd64":    true,
	"netbsd/arm":      true,
	"openbsd/386":     true,
	"openbsd/amd64":   true,
	"openbsd/arm":     true,
	"plan9/386":       false,
	"plan9/amd64":     false,
	"plan9/arm":       false,
	"solaris/amd64":   true,
	"windows/386":     true,
	"windows/amd64":   true,
	"windows/arm":     false,
}

// Version prints the Go version.
func cmdversion() {
	xflagparse(0)
	xprintf("%s\n", findgoversion())
}

// Banner prints the 'now you've installed Go' banner.
func cmdbanner() {
	xflagparse(0)
	banner()
}

// Clean deletes temporary objects.
func cmdclean() {
	xflagparse(0)
	clean()
}
