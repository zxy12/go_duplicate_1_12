package main

import (
	"fmt"
	"os"
	"strings"
)

func banner() {
	if vflag > 0 {
		xprintf("\n")
	}
	xprintf("---\n")
	xprintf("Installed Go for %s/%s in %s\n", goos, goarch, goroot)
	xprintf("Installed commands in %s\n", gobin)

	if !xsamefile(goroot_final, goroot) {
		// If the files are to be moved, don't check that gobin
		// is on PATH; assume they know what they are doing.
	} else if gohostos == "plan9" {
		// Check that gobin is bound before /bin.
		pid := strings.Replace(readfile("#c/pid"), " ", "", -1)
		ns := fmt.Sprintf("/proc/%s/ns", pid)
		if !strings.Contains(readfile(ns), fmt.Sprintf("bind -b %s /bin", gobin)) {
			xprintf("*** You need to bind %s before /bin.\n", gobin)
		}
	} else {
		// Check that gobin appears in $PATH.
		pathsep := ":"
		if gohostos == "windows" {
			pathsep = ";"
		}
		if !strings.Contains(pathsep+os.Getenv("PATH")+pathsep, pathsep+gobin+pathsep) {
			xprintf("*** You need to add %s to your PATH.\n", gobin)
		}
	}

	if !xsamefile(goroot_final, goroot) {
		xprintf("\n"+
			"The binaries expect %s to be copied or moved to %s\n",
			goroot, goroot_final)
	}
}
