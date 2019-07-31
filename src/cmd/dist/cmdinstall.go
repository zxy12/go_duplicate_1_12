package main

import (
	"flag"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

func _cmdinstall() {
	xflagparse(-1)

	if flag.NArg() == 0 {
		install(defaulttarg())
	}

	for _, arg := range flag.Args() {
		install(arg)
	}
}

func defaulttarg() string {
	// xgetwd might return a path with symlinks fully resolved, and if
	// there happens to be symlinks in goroot, then the hasprefix test
	// will never succeed. Instead, we use xrealwd to get a canonical
	// goroot/src before the comparison to avoid this problem.
	pwd := xgetwd()
	src := pathf("%s/src/", goroot)
	real_src := xrealwd(src)
	if !strings.HasPrefix(pwd, real_src) {
		fatalf("current directory %s is not under %s", pwd, real_src)
	}
	pwd = pwd[len(real_src):]
	// guard against xrealwd returning the directory without the trailing /
	pwd = strings.TrimPrefix(pwd, "/")

	return pwd
}

func install(dir string) {
	<-startInstall(dir)
}

func startInstall(dir string) chan struct{} {
	installedMu.Lock()
	ch := installed[dir]
	if ch == nil {
		ch = make(chan struct{})
		installed[dir] = ch
		go runInstall(dir, ch)
	}
	installedMu.Unlock()
	return ch
}

// runInstall installs the library, package, or binary associated with dir,
// which is relative to $GOROOT/src.
func runInstall(dir string, ch chan struct{}) {
	if dir == "net" || dir == "os/user" || dir == "crypto/x509" {
		fatalf("go_bootstrap cannot depend on cgo package %s", dir)
	}

	defer close(ch)

	if dir == "unsafe" {
		return
	}

	if vflag > 0 {
		if goos != gohostos || goarch != gohostarch {
			errprintf("%s (%s/%s)\n", dir, goos, goarch)
		} else {
			errprintf("%s\n", dir)
		}
	}

	workdir := pathf("%s/%s", workdir, dir)

	var clean []string
	defer func() {
		for _, name := range clean {
			xremove(name)
		}
	}()

	// path = full path to dir.
	path := pathf("%s/src/%s", goroot, dir)
	name := filepath.Base(dir)

	ispkg := !strings.HasPrefix(dir, "cmd/") || strings.Contains(dir, "/internal/")

	// Start final link command line.
	// Note: code below knows that link.p[targ] is the target.
	var (
		link      []string
		targ      int
		ispackcmd bool
	)
	if ispkg {
		// Go library (package).
		ispackcmd = true
		link = []string{"pack", pathf("%s/pkg/%s_%s/%s.a", goroot, goos, goarch, dir)}
		targ = len(link) - 1
		xmkdirall(filepath.Dir(link[targ]))
	} else {
		// Go command.
		elem := name
		if elem == "go" {
			elem = "go_bootstrap"
		}
		link = []string{pathf("%s/link", tooldir), "-o", pathf("%s/%s%s", tooldir, elem, exe)}
		targ = len(link) - 1
	}
	_p(1, "link=", dir, link, "ttarg=", link[targ])
	ttarg := mtime(link[targ])

	// Gather files that are sources for this target.
	// Everything in that directory, and any target-specific
	// additions.
	files := xreaddir(path)

	// Remove files beginning with . or _,
	// which are likely to be editor temporary files.
	// This is the same heuristic build.ScanDir uses.
	// There do exist real C files beginning with _,
	// so limit that check to just Go files.
	files = filter(files, func(p string) bool {
		return !strings.HasPrefix(p, ".") && (!strings.HasPrefix(p, "_") || !strings.HasSuffix(p, ".go"))
	})

	for _, dt := range deptab {
		if dir == dt.prefix || strings.HasSuffix(dt.prefix, "/") && strings.HasPrefix(dir, dt.prefix) {
			for _, p := range dt.dep {
				p = os.ExpandEnv(p)
				files = append(files, p)
			}
		}
	}
	files = uniq(files)

	// Convert to absolute paths.
	for i, p := range files {
		if !filepath.IsAbs(p) {
			files[i] = pathf("%s/%s", path, p)
		}
	}

	// Is the target up-to-date?
	var gofiles, sfiles, missing []string
	stale := rebuildall
	files = filter(files, func(p string) bool {
		for _, suf := range depsuffix {
			if strings.HasSuffix(p, suf) {
				goto ok
			}
		}
		return false
	ok:
		t := mtime(p)
		if !t.IsZero() && !strings.HasSuffix(p, ".a") && !shouldbuild(p, dir) {
			return false
		}
		if strings.HasSuffix(p, ".go") {
			gofiles = append(gofiles, p)
		} else if strings.HasSuffix(p, ".s") {
			sfiles = append(sfiles, p)
		}
		if t.After(ttarg) {
			stale = true
		}
		if t.IsZero() {
			missing = append(missing, p)
		}
		return true
	})

	// If there are no files to compile, we're done.
	if len(files) == 0 {
		return
	}

	if !stale {
		return
	}

	// For package runtime, copy some files into the work space.
	if dir == "runtime" {
		xmkdirall(pathf("%s/pkg/include", goroot))
		// For use by assembly and C files.
		copyfile(pathf("%s/pkg/include/textflag.h", goroot),
			pathf("%s/src/runtime/textflag.h", goroot), 0)
		copyfile(pathf("%s/pkg/include/funcdata.h", goroot),
			pathf("%s/src/runtime/funcdata.h", goroot), 0)
		copyfile(pathf("%s/pkg/include/asm_ppc64x.h", goroot),
			pathf("%s/src/runtime/asm_ppc64x.h", goroot), 0)
	}

	// Generate any missing files; regenerate existing ones.
	for _, p := range files {
		elem := filepath.Base(p)
		for _, gt := range gentab {
			if gt.gen == nil {
				continue
			}
			if strings.HasPrefix(elem, gt.nameprefix) {
				if vflag > 1 {
					errprintf("generate %s\n", p)
				}
				gt.gen(path, p)
				// Do not add generated file to clean list.
				// In runtime, we want to be able to
				// build the package with the go tool,
				// and it assumes these generated files already
				// exist (it does not know how to build them).
				// The 'clean' command can remove
				// the generated files.
				goto built
			}
		}
		// Did not rebuild p.
		if find(p, missing) >= 0 {
			fatalf("missing file %s", p)
		}
	built:
	}

	// Make sure dependencies are installed.
	var deps []string
	for _, p := range gofiles {
		deps = append(deps, readimports(p)...)
	}
	for _, dir1 := range deps {
		startInstall(dir1)
	}
	for _, dir1 := range deps {
		install(dir1)
	}

	if goos != gohostos || goarch != gohostarch {
		// We've generated the right files; the go command can do the build.
		if vflag > 1 {
			errprintf("skip build for cross-compile %s\n", dir)
		}
		return
	}

	asmArgs := []string{
		pathf("%s/asm", tooldir),
		"-I", workdir,
		"-I", pathf("%s/pkg/include", goroot),
		"-D", "GOOS_" + goos,
		"-D", "GOARCH_" + goarch,
		"-D", "GOOS_GOARCH_" + goos + "_" + goarch,
	}
	if goarch == "mips" || goarch == "mipsle" {
		// Define GOMIPS_value from gomips.
		asmArgs = append(asmArgs, "-D", "GOMIPS_"+gomips)
	}
	if goarch == "mips64" || goarch == "mipsle64" {
		// Define GOMIPS64_value from gomips64.
		asmArgs = append(asmArgs, "-D", "GOMIPS64_"+gomips64)
	}
	goasmh := pathf("%s/go_asm.h", workdir)

	// Collect symabis from assembly code.
	var symabis string
	if len(sfiles) > 0 {
		symabis = pathf("%s/symabis", workdir)
		var wg sync.WaitGroup
		asmabis := append(asmArgs[:len(asmArgs):len(asmArgs)], "-gensymabis", "-o", symabis)
		asmabis = append(asmabis, sfiles...)
		if err := ioutil.WriteFile(goasmh, nil, 0666); err != nil {
			fatalf("cannot write empty go_asm.h: %s", err)
		}
		bgrun(&wg, path, asmabis...)
		bgwait(&wg)
	}

	var archive string
	// The next loop will compile individual non-Go files.
	// Hand the Go files to the compiler en masse.
	// For packages containing assembly, this writes go_asm.h, which
	// the assembly files will need.
	pkg := dir
	if strings.HasPrefix(dir, "cmd/") && strings.Count(dir, "/") == 1 {
		pkg = "main"
	}
	b := pathf("%s/_go_.a", workdir)
	clean = append(clean, b)
	if !ispackcmd {
		link = append(link, b)
	} else {
		archive = b
	}

	// Compile Go code.
	compile := []string{pathf("%s/compile", tooldir), "-std", "-pack", "-o", b, "-p", pkg}
	if gogcflags != "" {
		compile = append(compile, strings.Fields(gogcflags)...)
	}
	if dir == "runtime" {
		compile = append(compile, "-+")
	}
	if len(sfiles) > 0 {
		compile = append(compile, "-asmhdr", goasmh)
	}
	if symabis != "" {
		compile = append(compile, "-symabis", symabis)
	}
	if dir == "runtime" || dir == "runtime/internal/atomic" {
		// These packages define symbols referenced by
		// assembly in other packages. In cmd/go, we work out
		// the exact details. For bootstrapping, just tell the
		// compiler to generate ABI wrappers for everything.
		compile = append(compile, "-allabis")
	}

	compile = append(compile, gofiles...)
	var wg sync.WaitGroup
	// We use bgrun and immediately wait for it instead of calling run() synchronously.
	// This executes all jobs through the bgwork channel and allows the process
	// to exit cleanly in case an error occurs.
	bgrun(&wg, path, compile...)
	bgwait(&wg)

	// Compile the files.
	for _, p := range sfiles {
		// Assembly file for a Go package.
		compile := asmArgs[:len(asmArgs):len(asmArgs)]

		doclean := true
		b := pathf("%s/%s", workdir, filepath.Base(p))

		// Change the last character of the output file (which was c or s).
		b = b[:len(b)-1] + "o"
		compile = append(compile, "-o", b, p)
		bgrun(&wg, path, compile...)

		link = append(link, b)
		if doclean {
			clean = append(clean, b)
		}
	}
	bgwait(&wg)

	if ispackcmd {
		xremove(link[targ])
		dopack(link[targ], archive, link[targ+1:])
		return
	}

	// Remove target before writing it.
	xremove(link[targ])
	bgrun(&wg, "", link...)
	bgwait(&wg)
}
