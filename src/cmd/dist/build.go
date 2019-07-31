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

/*
 * command implementations
 */

// The env command prints the default environment.
func cmdenv() {
	_cmdenv()
}

func cmdlist() {
	_cmdlist()
}
