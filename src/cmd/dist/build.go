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
