package main

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// The bootstrap command runs a build from scratch,
// stopping at having installed the go_bootstrap command.
//
// WARNING: This command runs after cmd/dist is built with Go 1.4.
// It rebuilds and installs cmd/dist with the new toolchain, so other
// commands (like "go tool dist test" in run.bash) can rely on bug fixes
// made since Go 1.4, but this function cannot. In particular, the uses
// of os/exec in this function cannot assume that
//  cmd.Env = append(os.Environ(), "X=Y")
// sets $X to Y in the command's environment. That guarantee was
// added after Go 1.4, and in fact in Go 1.4 it was typically the opposite:
// if $X was already present in os.Environ(), most systems preferred
// that setting, not the new one.
func _cmdbootstrap() {
	timelog("start", "dist bootstrap")
	defer timelog("end", "dist bootstrap")

	var noBanner bool
	var debug bool
	flag.BoolVar(&rebuildall, "a", rebuildall, "rebuild all")
	flag.BoolVar(&debug, "d", debug, "enable debugging of bootstrap process")
	flag.BoolVar(&noBanner, "no-banner", noBanner, "do not print banner")

	xflagparse(0)

	if debug {
		// cmd/buildid is used in debug mode.
		toolchain = append(toolchain, "cmd/buildid")
	}

	if isdir(pathf("%s/src/pkg", goroot)) {
		fatalf("\n\n"+
			"The Go package sources have moved to $GOROOT/src.\n"+
			"*** %s still exists. ***\n"+
			"It probably contains stale files that may confuse the build.\n"+
			"Please (check what's there and) remove it and try again.\n"+
			"See https://golang.org/s/go14nopkg\n",
			pathf("%s/src/pkg", goroot))
	}

	if rebuildall {
		clean()
	}

	setup()

	timelog("build", "toolchain1")
	checkCC()
	bootstrapBuildTools()

	// Remember old content of $GOROOT/bin for comparison below.
	oldBinFiles, _ := filepath.Glob(pathf("%s/bin/*", goroot))

	// For the main bootstrap, building for host os/arch.
	oldgoos = goos
	oldgoarch = goarch
	goos = gohostos
	goarch = gohostarch
	os.Setenv("GOHOSTARCH", gohostarch)
	os.Setenv("GOHOSTOS", gohostos)
	os.Setenv("GOARCH", goarch)
	os.Setenv("GOOS", goos)

	timelog("build", "go_bootstrap")
	xprintf("Building Go bootstrap cmd/go (go_bootstrap) using Go toolchain1.\n")

	install("runtime") // dependency not visible in sources; also sets up textflag.h

	install("cmd/go")
	if vflag > 0 {
		xprintf("\n")
	}

	gogcflags = os.Getenv("GO_GCFLAGS") // we were using $BOOT_GO_GCFLAGS until now
	goldflags = os.Getenv("GO_LDFLAGS")
	goBootstrap := pathf("%s/go_bootstrap", tooldir)
	cmdGo := pathf("%s/go", gobin)
	if debug {
		run("", ShowOutput|CheckExit, pathf("%s/compile", tooldir), "-V=full")
		copyfile(pathf("%s/compile1", tooldir), pathf("%s/compile", tooldir), writeExec)
	}

	// To recap, so far we have built the new toolchain
	// (cmd/asm, cmd/cgo, cmd/compile, cmd/link)
	// using Go 1.4's toolchain and go command.
	// Then we built the new go command (as go_bootstrap)
	// using the new toolchain and our own build logic (above).
	//
	//  toolchain1 = mk(new toolchain, go1.4 toolchain, go1.4 cmd/go)
	//  go_bootstrap = mk(new cmd/go, toolchain1, cmd/dist)
	//
	// The toolchain1 we built earlier is built from the new sources,
	// but because it was built using cmd/go it has no build IDs.
	// The eventually installed toolchain needs build IDs, so we need
	// to do another round:
	//
	//  toolchain2 = mk(new toolchain, toolchain1, go_bootstrap)
	//
	timelog("build", "toolchain2")
	if vflag > 0 {
		xprintf("\n")
	}
	xprintf("Building Go toolchain2 using go_bootstrap and Go toolchain1.\n")
	os.Setenv("CC", compilerEnvLookup(defaultcc, goos, goarch))

	_pf(1, "goBootstrap=[%s,%v]", goBootstrap, toolchain)

	goInstall(goBootstrap, append([]string{"-i"}, toolchain...)...)
	if debug {
		run("", ShowOutput|CheckExit, pathf("%s/compile", tooldir), "-V=full")
		run("", ShowOutput|CheckExit, pathf("%s/buildid", tooldir), pathf("%s/pkg/%s_%s/runtime/internal/sys.a", goroot, goos, goarch))
		copyfile(pathf("%s/compile2", tooldir), pathf("%s/compile", tooldir), writeExec)
	}

	// Toolchain2 should be semantically equivalent to toolchain1,
	// but it was built using the new compilers instead of the Go 1.4 compilers,
	// so it should at the least run faster. Also, toolchain1 had no build IDs
	// in the binaries, while toolchain2 does. In non-release builds, the
	// toolchain's build IDs feed into constructing the build IDs of built targets,
	// so in non-release builds, everything now looks out-of-date due to
	// toolchain2 having build IDs - that is, due to the go command seeing
	// that there are new compilers. In release builds, the toolchain's reported
	// version is used in place of the build ID, and the go command does not
	// see that change from toolchain1 to toolchain2, so in release builds,
	// nothing looks out of date.
	// To keep the behavior the same in both non-release and release builds,
	// we force-install everything here.
	//
	//  toolchain3 = mk(new toolchain, toolchain2, go_bootstrap)
	//
	timelog("build", "toolchain3")
	if vflag > 0 {
		xprintf("\n")
	}
	xprintf("Building Go toolchain3 using go_bootstrap and Go toolchain2.\n")
	goInstall(goBootstrap, append([]string{"-a", "-i"}, toolchain...)...)
	if debug {
		run("", ShowOutput|CheckExit, pathf("%s/compile", tooldir), "-V=full")
		run("", ShowOutput|CheckExit, pathf("%s/buildid", tooldir), pathf("%s/pkg/%s_%s/runtime/internal/sys.a", goroot, goos, goarch))
		copyfile(pathf("%s/compile3", tooldir), pathf("%s/compile", tooldir), writeExec)
	}
	checkNotStale(goBootstrap, append(toolchain, "runtime/internal/sys")...)

	if goos == oldgoos && goarch == oldgoarch {
		// Common case - not setting up for cross-compilation.
		timelog("build", "toolchain")
		if vflag > 0 {
			xprintf("\n")
		}
		xprintf("Building packages and commands for %s/%s.\n", goos, goarch)
	} else {
		// GOOS/GOARCH does not match GOHOSTOS/GOHOSTARCH.
		// Finish GOHOSTOS/GOHOSTARCH installation and then
		// run GOOS/GOARCH installation.
		timelog("build", "host toolchain")
		if vflag > 0 {
			xprintf("\n")
		}
		xprintf("Building packages and commands for host, %s/%s.\n", goos, goarch)
		goInstall(goBootstrap, "std", "cmd")
		checkNotStale(goBootstrap, "std", "cmd")
		checkNotStale(cmdGo, "std", "cmd")

		timelog("build", "target toolchain")
		if vflag > 0 {
			xprintf("\n")
		}
		goos = oldgoos
		goarch = oldgoarch
		os.Setenv("GOOS", goos)
		os.Setenv("GOARCH", goarch)
		os.Setenv("CC", compilerEnvLookup(defaultcc, goos, goarch))
		xprintf("Building packages and commands for target, %s/%s.\n", goos, goarch)
	}
	targets := []string{"std", "cmd"}
	if goos == "js" && goarch == "wasm" {
		// Skip the cmd tools for js/wasm. They're not usable.
		targets = targets[:1]
	}
	goInstall(goBootstrap, targets...)
	checkNotStale(goBootstrap, targets...)
	checkNotStale(cmdGo, targets...)
	if debug {
		run("", ShowOutput|CheckExit, pathf("%s/compile", tooldir), "-V=full")
		run("", ShowOutput|CheckExit, pathf("%s/buildid", tooldir), pathf("%s/pkg/%s_%s/runtime/internal/sys.a", goroot, goos, goarch))
		checkNotStale(goBootstrap, append(toolchain, "runtime/internal/sys")...)
		copyfile(pathf("%s/compile4", tooldir), pathf("%s/compile", tooldir), writeExec)
	}

	// Check that there are no new files in $GOROOT/bin other than
	// go and gofmt and $GOOS_$GOARCH (target bin when cross-compiling).
	binFiles, _ := filepath.Glob(pathf("%s/bin/*", goroot))
	ok := map[string]bool{}
	for _, f := range oldBinFiles {
		ok[f] = true
	}
	for _, f := range binFiles {
		elem := strings.TrimSuffix(filepath.Base(f), ".exe")
		if !ok[f] && elem != "go" && elem != "gofmt" && elem != goos+"_"+goarch {
			fatalf("unexpected new file in $GOROOT/bin: %s", elem)
		}
	}

	// Remove go_bootstrap now that we're done.
	xremove(pathf("%s/go_bootstrap", tooldir))

	// Print trailing banner unless instructed otherwise.
	if !noBanner {
		banner()
	}
}

// setup sets up the tree for the initial build.
func setup() {
	// Create bin directory.
	if p := pathf("%s/bin", goroot); !isdir(p) {
		xmkdir(p)
	}

	// Create package directory.
	if p := pathf("%s/pkg", goroot); !isdir(p) {
		xmkdir(p)
	}

	p := pathf("%s/pkg/%s_%s", goroot, gohostos, gohostarch)
	if rebuildall {
		xremoveall(p)
	}
	xmkdirall(p)

	if goos != gohostos || goarch != gohostarch {
		p := pathf("%s/pkg/%s_%s", goroot, goos, goarch)
		if rebuildall {
			xremoveall(p)
		}
		xmkdirall(p)
	}

	// Create object directory.
	// We used to use it for C objects.
	// Now we use it for the build cache, to separate dist's cache
	// from any other cache the user might have.
	p = pathf("%s/pkg/obj/go-build", goroot)
	if rebuildall {
		xremoveall(p)
	}
	xmkdirall(p)

	// Create tool directory.
	// We keep it in pkg/, just like the object directory above.
	if rebuildall {
		xremoveall(tooldir)
	}
	xmkdirall(tooldir)

	// Remove tool binaries from before the tool/gohostos_gohostarch
	xremoveall(pathf("%s/bin/tool", goroot))

	// Remove old pre-tool binaries.
	for _, old := range oldtool {
		xremove(pathf("%s/bin/%s", goroot, old))
	}

	// If $GOBIN is set and has a Go compiler, it must be cleaned.
	for _, char := range "56789" {
		if isfile(pathf("%s/%c%s", gobin, char, "g")) {
			for _, old := range oldtool {
				xremove(pathf("%s/%s", gobin, old))
			}
			break
		}
	}

	// For release, make sure excluded things are excluded.
	goversion := findgoversion()
	if strings.HasPrefix(goversion, "release.") || (strings.HasPrefix(goversion, "go") && !strings.Contains(goversion, "beta")) {
		for _, dir := range unreleased {
			if p := pathf("%s/%s", goroot, dir); isdir(p) {
				fatalf("%s should not exist in release build", p)
			}
		}
	}
}

func checkCC() {
	if !needCC() {
		return
	}
	if output, err := exec.Command(defaultcc[""], "--help").CombinedOutput(); err != nil {
		outputHdr := ""
		if len(output) > 0 {
			outputHdr = "\nCommand output:\n\n"
		}
		fatalf("cannot invoke C compiler %q: %v\n\n"+
			"Go needs a system C compiler for use with cgo.\n"+
			"To set a C compiler, set CC=the-compiler.\n"+
			"To disable cgo, set CGO_ENABLED=0.\n%s%s", defaultcc[""], err, outputHdr, output)
	}
}

func needCC() bool {
	switch os.Getenv("CGO_ENABLED") {
	case "1":
		return true
	case "0":
		return false
	}
	return cgoEnabled[gohostos+"/"+gohostarch]
}
