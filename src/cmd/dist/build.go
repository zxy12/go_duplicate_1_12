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
	"os"
	"path/filepath"
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

// xinit handles initialization of the various global state, like goroot and goarch.
func xinit() {
	b := os.Getenv("GOROOT")
	if b == "" {
		fatalf("$GOROOT must be set")
	}
	goroot = filepath.Clean(b)

	b = os.Getenv("GOROOT_FINAL")
	if b == "" {
		b = goroot
	}
	goroot_final = b

	b = os.Getenv("GOBIN")
	if b == "" {
		b = pathf("%s/bin", goroot)
	}
	gobin = b

	b = os.Getenv("GOOS")
	if b == "" {
		b = gohostos
	}
	goos = b
	if find(goos, okgoos) < 0 {
		fatalf("unknown $GOOS %s", goos)
	}

	b = os.Getenv("GOARM")
	if b == "" {
		b = xgetgoarm()
	}
	goarm = b

	b = os.Getenv("GO386")
	if b == "" {
		if cansse2() {
			b = "sse2"
		} else {
			b = "387"
		}
	}
	go386 = b

	b = os.Getenv("GOMIPS")
	if b == "" {
		b = "hardfloat"
	}
	gomips = b

	b = os.Getenv("GOMIPS64")
	if b == "" {
		b = "hardfloat"
	}
	gomips64 = b

	if p := pathf("%s/src/all.bash", goroot); !isfile(p) {
		fatalf("$GOROOT is not set correctly or not exported\n"+
			"\tGOROOT=%s\n"+
			"\t%s does not exist", goroot, p)
	}

	b = os.Getenv("GOHOSTARCH")
	if b != "" {
		gohostarch = b
	}
	if find(gohostarch, okgoarch) < 0 {
		fatalf("unknown $GOHOSTARCH %s", gohostarch)
	}

	b = os.Getenv("GOARCH")
	if b == "" {
		b = gohostarch
	}
	goarch = b
	if find(goarch, okgoarch) < 0 {
		fatalf("unknown $GOARCH %s", goarch)
	}

	b = os.Getenv("GO_EXTLINK_ENABLED")
	if b != "" {
		if b != "0" && b != "1" {
			fatalf("unknown $GO_EXTLINK_ENABLED %s", b)
		}
		goextlinkenabled = b
	}

	gogcflags = os.Getenv("BOOT_GO_GCFLAGS")

	cc, cxx := "gcc", "g++"
	if defaultclang {
		cc, cxx = "clang", "clang++"
	}
	defaultcc = compilerEnv("CC", cc)
	defaultcxx = compilerEnv("CXX", cxx)

	defaultcflags = os.Getenv("CFLAGS")
	defaultldflags = os.Getenv("LDFLAGS")

	b = os.Getenv("PKG_CONFIG")
	if b == "" {
		b = "pkg-config"
	}
	defaultpkgconfig = b

	// For tools being invoked but also for os.ExpandEnv.
	os.Setenv("GO386", go386)
	os.Setenv("GOARCH", goarch)
	os.Setenv("GOARM", goarm)
	os.Setenv("GOHOSTARCH", gohostarch)
	os.Setenv("GOHOSTOS", gohostos)
	os.Setenv("GOOS", goos)
	os.Setenv("GOMIPS", gomips)
	os.Setenv("GOMIPS64", gomips64)
	os.Setenv("GOROOT", goroot)
	os.Setenv("GOROOT_FINAL", goroot_final)

	// Use a build cache separate from the default user one.
	// Also one that will be wiped out during startup, so that
	// make.bash really does start from a clean slate.
	os.Setenv("GOCACHE", pathf("%s/pkg/obj/go-build", goroot))

	// Make the environment more predictable.
	os.Setenv("LANG", "C")
	os.Setenv("LANGUAGE", "en_US.UTF8")

	workdir = xworkdir()
	xatexit(rmworkdir)

	tooldir = pathf("%s/pkg/tool/%s_%s", goroot, gohostos, gohostarch)
}

// find reports the first index of p in l[0:n], or else -1.
func find(p string, l []string) int {
	for i, s := range l {
		if p == s {
			return i
		}
	}
	return -1
}

// rmworkdir deletes the work directory.
func rmworkdir() {
	if vflag > 1 {
		errprintf("rm -rf %s\n", workdir)
	}
	xremoveall(workdir)
}

// compilerEnv returns a map from "goos/goarch" to the
// compiler setting to use for that platform.
// The entry for key "" covers any goos/goarch not explicitly set in the map.
// For example, compilerEnv("CC", "gcc") returns the C compiler settings
// read from $CC, defaulting to gcc.
//
// The result is a map because additional environment variables
// can be set to change the compiler based on goos/goarch settings.
// The following applies to all envNames but CC is assumed to simplify
// the presentation.
//
// If no environment variables are set, we use def for all goos/goarch.
// $CC, if set, applies to all goos/goarch but is overridden by the following.
// $CC_FOR_TARGET, if set, applies to all goos/goarch except gohostos/gohostarch,
// but is overridden by the following.
// If gohostos=goos and gohostarch=goarch, then $CC_FOR_TARGET applies even for gohostos/gohostarch.
// $CC_FOR_goos_goarch, if set, applies only to goos/goarch.
func compilerEnv(envName, def string) map[string]string {
	m := map[string]string{"": def}

	if env := os.Getenv(envName); env != "" {
		m[""] = env
	}
	if env := os.Getenv(envName + "_FOR_TARGET"); env != "" {
		if gohostos != goos || gohostarch != goarch {
			m[gohostos+"/"+gohostarch] = m[""]
		}
		m[""] = env
	}

	for _, goos := range okgoos {
		for _, goarch := range okgoarch {
			if env := os.Getenv(envName + "_FOR_" + goos + "_" + goarch); env != "" {
				m[goos+"/"+goarch] = env
			}
		}
	}

	return m
}
