// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// pathf is fmt.Sprintf for generating paths
// (on windows it turns / into \ after the printf).
func pathf(format string, args ...interface{}) string {
	return filepath.Clean(fmt.Sprintf(format, args...))
}

// filter returns a slice containing the elements x from list for which f(x) == true.
func filter(list []string, f func(string) bool) []string {
	var out []string
	for _, x := range list {
		if f(x) {
			out = append(out, x)
		}
	}
	return out
}

// uniq returns a sorted slice containing the unique elements of list.
func uniq(list []string) []string {
	out := make([]string, len(list))
	copy(out, list)
	sort.Strings(out)
	keep := out[:0]
	for _, x := range out {
		if len(keep) == 0 || keep[len(keep)-1] != x {
			keep = append(keep, x)
		}
	}
	return keep
}

const (
	CheckExit = 1 << iota
	ShowOutput
	Background
)

var outputLock sync.Mutex

// run runs the command line cmd in dir.
// If mode has ShowOutput set and Background unset, run passes cmd's output to
// stdout/stderr directly. Otherwise, run returns cmd's output as a string.
// If mode has CheckExit set and the command fails, run calls fatalf.
// If mode has Background set, this command is being run as a
// Background job. Only bgrun should use the Background mode,
// not other callers.
func run(dir string, mode int, cmd ...string) string {
	if vflag > 1 {
		errprintf("run: %s\n", strings.Join(cmd, " "))
	}

	_pf(1, "exec: %s/%s\n", dir, strings.Join(cmd, " "))
	xcmd := exec.Command(cmd[0], cmd[1:]...)
	xcmd.Dir = dir
	var data []byte
	var err error

	// If we want to show command output and this is not
	// a background command, assume it's the only thing
	// running, so we can just let it write directly stdout/stderr
	// as it runs without fear of mixing the output with some
	// other command's output. Not buffering lets the output
	// appear as it is printed instead of once the command exits.
	// This is most important for the invocation of 'go1.4 build -v bootstrap/...'.
	if mode&(Background|ShowOutput) == ShowOutput {
		xcmd.Stdout = os.Stdout
		xcmd.Stderr = os.Stderr
		err = xcmd.Run()
	} else {
		data, err = xcmd.CombinedOutput()
	}
	if err != nil && mode&CheckExit != 0 {
		outputLock.Lock()
		if len(data) > 0 {
			xprintf("%s\n", data)
		}
		outputLock.Unlock()
		if mode&Background != 0 {
			// Prevent fatalf from waiting on our own goroutine's
			// bghelper to exit:
			bghelpers.Done()
		}
		fatalf("FAILED: %v: %v", strings.Join(cmd, " "), err)
	}
	if mode&ShowOutput != 0 {
		outputLock.Lock()
		os.Stdout.Write(data)
		outputLock.Unlock()
	}
	if vflag > 2 {
		errprintf("run: %s DONE\n", strings.Join(cmd, " "))
	}
	return string(data)
}

var maxbg = 4 /* maximum number of jobs to run at once */

var (
	bgwork = make(chan func(), 1e5)

	bghelpers sync.WaitGroup

	dieOnce sync.Once // guards close of dying
	dying   = make(chan struct{})
)

func bginit() {
	bghelpers.Add(maxbg)
	for i := 0; i < maxbg; i++ {
		go bghelper()
	}
}

func bghelper() {
	defer bghelpers.Done()
	for {
		select {
		case <-dying:
			return
		case w := <-bgwork:
			// Dying takes precedence over doing more work.
			select {
			case <-dying:
				return
			default:
				w()
			}
		}
	}
}

// bgrun is like run but runs the command in the background.
// CheckExit|ShowOutput mode is implied (since output cannot be returned).
// bgrun adds 1 to wg immediately, and calls Done when the work completes.
func bgrun(wg *sync.WaitGroup, dir string, cmd ...string) {
	wg.Add(1)
	bgwork <- func() {
		defer wg.Done()
		run(dir, CheckExit|ShowOutput|Background, cmd...)
	}
}

// bgwait waits for pending bgruns to finish.
// bgwait must be called from only a single goroutine at a time.
func bgwait(wg *sync.WaitGroup) {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-dying:
	}
}

// xgetwd returns the current directory.
func xgetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		fatalf("%s", err)
	}
	return wd
}

// xrealwd returns the 'real' name for the given path.
// real is defined as what xgetwd returns in that directory.
func xrealwd(path string) string {
	old := xgetwd()
	if err := os.Chdir(path); err != nil {
		fatalf("chdir %s: %v", path, err)
	}
	real := xgetwd()
	if err := os.Chdir(old); err != nil {
		fatalf("chdir %s: %v", old, err)
	}
	return real
}

// isdir reports whether p names an existing directory.
func isdir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// isfile reports whether p names an existing file.
func isfile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

// mtime returns the modification time of the file p.
func mtime(p string) time.Time {
	fi, err := os.Stat(p)
	if err != nil {
		return time.Time{}
	}
	return fi.ModTime()
}

// readfile returns the content of the named file.
func readfile(file string) string {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		fatalf("%v", err)
	}
	return string(data)
}

const (
	writeExec = 1 << iota
	writeSkipSame
)

// writefile writes text to the named file, creating it if needed.
// if exec is non-zero, marks the file as executable.
// If the file already exists and has the expected content,
// it is not rewritten, to avoid changing the time stamp.
func writefile(text, file string, flag int) {
	new := []byte(text)
	if flag&writeSkipSame != 0 {
		old, err := ioutil.ReadFile(file)
		if err == nil && bytes.Equal(old, new) {
			return
		}
	}
	mode := os.FileMode(0666)
	if flag&writeExec != 0 {
		mode = 0777
	}
	err := ioutil.WriteFile(file, new, mode)
	if err != nil {
		fatalf("%v", err)
	}
}

// xmkdir creates the directory p.
func xmkdir(p string) {
	err := os.Mkdir(p, 0777)
	if err != nil {
		fatalf("%v", err)
	}
}

// xmkdirall creates the directory p and its parents, as needed.
func xmkdirall(p string) {
	err := os.MkdirAll(p, 0777)
	if err != nil {
		fatalf("%v", err)
	}
}

// xremove removes the file p.
func xremove(p string) {

	_p(1, "[rm file:]", p)
	if vflag > 2 {
		errprintf("rm %s\n", p)
	}
	os.Remove(p)
}

// xremoveall removes the file or directory tree rooted at p.
func xremoveall(p string) {
	if vflag > 2 {
		errprintf("rm -r %s\n", p)
	}
	os.RemoveAll(p)
}

// xreaddir replaces dst with a list of the names of the files and subdirectories in dir.
// The names are relative to dir; they are not full paths.
func xreaddir(dir string) []string {
	f, err := os.Open(dir)
	if err != nil {
		fatalf("%v", err)
	}
	defer f.Close()
	names, err := f.Readdirnames(-1)
	if err != nil {
		fatalf("reading %s: %v", dir, err)
	}
	return names
}

// xreaddir replaces dst with a list of the names of the files in dir.
// The names are relative to dir; they are not full paths.
func xreaddirfiles(dir string) []string {
	f, err := os.Open(dir)
	if err != nil {
		fatalf("%v", err)
	}
	defer f.Close()
	infos, err := f.Readdir(-1)
	if err != nil {
		fatalf("reading %s: %v", dir, err)
	}
	var names []string
	for _, fi := range infos {
		if !fi.IsDir() {
			names = append(names, fi.Name())
		}
	}
	return names
}

// xworkdir creates a new temporary directory to hold object files
// and returns the name of that directory.
func xworkdir() string {
	name, err := ioutil.TempDir(os.Getenv("GOTMPDIR"), "go-tool-dist-")
	if err != nil {
		fatalf("%v", err)
	}
	return name
}

// fatalf prints an error message to standard error and exits.
func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "go tool dist: %s\n", fmt.Sprintf(format, args...))

	dieOnce.Do(func() { close(dying) })

	// Wait for background goroutines to finish,
	// so that exit handler that removes the work directory
	// is not fighting with active writes or open files.
	bghelpers.Wait()

	xexit(2)
}

var atexits []func()

// xexit exits the process with return code n.
func xexit(n int) {
	for i := len(atexits) - 1; i >= 0; i-- {
		atexits[i]()
	}
	os.Exit(n)
}

// xatexit schedules the exit-handler f to be run when the program exits.
func xatexit(f func()) {
	atexits = append(atexits, f)
}

// xprintf prints a message to standard output.
func xprintf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

// errprintf prints a message to standard output.
func errprintf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
}

// xsamefile reports whether f1 and f2 are the same file (or dir)
func xsamefile(f1, f2 string) bool {
	fi1, err1 := os.Stat(f1)
	fi2, err2 := os.Stat(f2)
	if err1 != nil || err2 != nil {
		return f1 == f2
	}
	return os.SameFile(fi1, fi2)
}

func xgetgoarm() string {
	if goos == "nacl" {
		// NaCl guarantees VFPv3 and is always cross-compiled.
		return "7"
	}
	if goos == "darwin" || goos == "android" {
		// Assume all darwin/arm and android devices have VFPv3.
		// These ports are also mostly cross-compiled, so it makes little
		// sense to auto-detect the setting.
		return "7"
	}
	if gohostarch != "arm" || goos != gohostos {
		// Conservative default for cross-compilation.
		return "5"
	}
	if goos == "freebsd" {
		// FreeBSD has broken VFP support.
		return "5"
	}

	// Try to exec ourselves in a mode to detect VFP support.
	// Seeing how far it gets determines which instructions failed.
	// The test is OS-agnostic.
	out := run("", 0, os.Args[0], "-check-goarm")
	v1ok := strings.Contains(out, "VFPv1 OK.")
	v3ok := strings.Contains(out, "VFPv3 OK.")

	if v1ok && v3ok {
		return "7"
	}
	if v1ok {
		return "6"
	}
	return "5"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// elfIsLittleEndian detects if the ELF file is little endian.
func elfIsLittleEndian(fn string) bool {
	// read the ELF file header to determine the endianness without using the
	// debug/elf package.
	file, err := os.Open(fn)
	if err != nil {
		fatalf("failed to open file to determine endianness: %v", err)
	}
	defer file.Close()
	var hdr [16]byte
	if _, err := io.ReadFull(file, hdr[:]); err != nil {
		fatalf("failed to read ELF header to determine endianness: %v", err)
	}
	// hdr[5] is EI_DATA byte, 1 is ELFDATA2LSB and 2 is ELFDATA2MSB
	switch hdr[5] {
	default:
		fatalf("unknown ELF endianness of %s: EI_DATA = %d", fn, hdr[5])
	case 1:
		return true
	case 2:
		return false
	}
	panic("unreachable")
}

// count is a flag.Value that is like a flag.Bool and a flag.Int.
// If used as -name, it increments the count, but -name=x sets the count.
// Used for verbose flag -v.
type count int

func (c *count) String() string {
	return fmt.Sprint(int(*c))
}

func (c *count) Set(s string) error {
	switch s {
	case "true":
		*c++
	case "false":
		*c = 0
	default:
		n, err := strconv.Atoi(s)
		if err != nil {
			return fmt.Errorf("invalid count %q", s)
		}
		*c = count(n)
	}
	return nil
}

func (c *count) IsBoolFlag() bool {
	return true
}

func xflagparse(maxargs int) {
	flag.Var((*count)(&vflag), "v", "verbosity")
	flag.Parse()
	if maxargs >= 0 && flag.NArg() > maxargs {
		flag.Usage()
	}
}

// Remove trailing spaces.
func chomp(s string) string {
	return strings.TrimRight(s, " \t\r\n")
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

func goInstall(goBinary string, args ...string) {
	installCmd := []string{goBinary, "install", "-gcflags=all=" + gogcflags, "-ldflags=all=" + goldflags}
	if vflag > 0 {
		installCmd = append(installCmd, "-v")
	}

	// Force only one process at a time on vx32 emulation.
	if gohostos == "plan9" && os.Getenv("sysname") == "vx32" {
		installCmd = append(installCmd, "-p=1")
	}

	run(goroot, ShowOutput|CheckExit, append(installCmd, args...)...)
}

func checkNotStale(goBinary string, targets ...string) {
	out := run(goroot, CheckExit,
		append([]string{
			goBinary,
			"list", "-gcflags=all=" + gogcflags, "-ldflags=all=" + goldflags,
			"-f={{if .Stale}}\tSTALE {{.ImportPath}}: {{.StaleReason}}{{end}}",
		}, targets...)...)
	if strings.Contains(out, "\tSTALE ") {
		os.Setenv("GODEBUG", "gocachehash=1")
		for _, target := range []string{"runtime/internal/sys", "cmd/dist", "cmd/link"} {
			if strings.Contains(out, "STALE "+target) {
				run(goroot, ShowOutput|CheckExit, goBinary, "list", "-f={{.ImportPath}} {{.Stale}}", target)
				break
			}
		}
		fatalf("unexpected stale targets reported by %s list -gcflags=\"%s\" -ldflags=\"%s\" for %v:\n%s", goBinary, gogcflags, goldflags, targets, out)
	}
}

// compilerEnvLookup returns the compiler settings for goos/goarch in map m.
func compilerEnvLookup(m map[string]string, goos, goarch string) string {
	if cc := m[goos+"/"+goarch]; cc != "" {
		return cc
	}
	return m[""]
}

// copy copies the file src to dst, via memory (so only good for small files).
func copyfile(dst, src string, flag int) {
	if vflag > 1 {
		errprintf("cp %s %s\n", src, dst)
	}
	writefile(readfile(src), dst, flag)
}

// shouldbuild reports whether we should build this file.
// It applies the same rules that are used with context tags
// in package go/build, except it's less picky about the order
// of GOOS and GOARCH.
// We also allow the special tag cmd_go_bootstrap.
// See ../go/bootstrap.go and package go/build.
func shouldbuild(file, dir string) bool {
	// Check file name for GOOS or GOARCH.
	name := filepath.Base(file)
	excluded := func(list []string, ok string) bool {
		for _, x := range list {
			if x == ok || ok == "android" && x == "linux" {
				continue
			}
			i := strings.Index(name, x)
			if i <= 0 || name[i-1] != '_' {
				continue
			}
			i += len(x)
			if i == len(name) || name[i] == '.' || name[i] == '_' {
				return true
			}
		}
		return false
	}
	if excluded(okgoos, goos) || excluded(okgoarch, goarch) {
		return false
	}

	// Omit test files.
	if strings.Contains(name, "_test") {
		return false
	}

	// Check file contents for // +build lines.
	for _, p := range strings.Split(readfile(file), "\n") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		code := p
		i := strings.Index(code, "//")
		if i > 0 {
			code = strings.TrimSpace(code[:i])
		}
		if code == "package documentation" {
			return false
		}
		if code == "package main" && dir != "cmd/go" && dir != "cmd/cgo" {
			return false
		}
		if !strings.HasPrefix(p, "//") {
			break
		}
		if !strings.Contains(p, "+build") {
			continue
		}
		fields := strings.Fields(p[2:])
		if len(fields) < 1 || fields[0] != "+build" {
			continue
		}
		for _, p := range fields[1:] {
			if matchfield(p) {
				goto fieldmatch
			}
		}
		return false
	fieldmatch:
	}

	return true
}

// dopack copies the package src to dst,
// appending the files listed in extra.
// The archive format is the traditional Unix ar format.
func dopack(dst, src string, extra []string) {
	bdst := bytes.NewBufferString(readfile(src))
	for _, file := range extra {
		b := readfile(file)
		// find last path element for archive member name
		i := strings.LastIndex(file, "/") + 1
		j := strings.LastIndex(file, `\`) + 1
		if i < j {
			i = j
		}
		fmt.Fprintf(bdst, "%-16.16s%-12d%-6d%-6d%-8o%-10d`\n", file[i:], 0, 0, 0, 0644, len(b))
		bdst.WriteString(b)
		if len(b)&1 != 0 {
			bdst.WriteByte(0)
		}
	}
	writefile(bdst.String(), dst, 0)
}

// matchfield reports whether the field (x,y,z) matches this build.
// all the elements in the field must be satisfied.
func matchfield(f string) bool {
	for _, tag := range strings.Split(f, ",") {
		if !matchtag(tag) {
			return false
		}
	}
	return true
}

// matchtag reports whether the tag (x or !x) matches this build.
func matchtag(tag string) bool {
	if tag == "" {
		return false
	}
	if tag[0] == '!' {
		if len(tag) == 1 || tag[1] == '!' {
			return false
		}
		return !matchtag(tag[1:])
	}
	return tag == "gc" || tag == goos || tag == goarch || tag == "cmd_go_bootstrap" || tag == "go1.1" || (goos == "android" && tag == "linux")
}
