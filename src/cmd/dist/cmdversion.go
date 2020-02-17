package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func branchtag(branch string) (tag string, precise bool) {
	log := run(goroot, CheckExit, "git", "log", "--decorate=full", "--format=format:%d", "master.."+branch)
	tag = branch
	for row, line := range strings.Split(log, "\n") {
		// Each line is either blank, or looks like
		//    (tag: refs/tags/go1.4rc2, refs/remotes/origin/release-branch.go1.4, refs/heads/release-branch.go1.4)
		// We need to find an element starting with refs/tags/.
		const s = " refs/tags/"
		i := strings.Index(line, s)
		if i < 0 {
			continue
		}
		// Trim off known prefix.
		line = line[i+len(s):]
		// The tag name ends at a comma or paren.
		j := strings.IndexAny(line, ",)")
		if j < 0 {
			continue // malformed line; ignore it
		}
		tag = line[:j]
		if row == 0 {
			precise = true // tag denotes HEAD
		}
		break
	}
	return
}

// isGitRepo reports whether the working directory is inside a Git repository.
func isGitRepo() bool {
	// NB: simply checking the exit code of `git rev-parse --git-dir` would
	// suffice here, but that requires deviating from the infrastructure
	// provided by `run`.
	gitDir := chomp(run(goroot, 0, "git", "rev-parse", "--git-dir"))
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(goroot, gitDir)
	}
	return isdir(gitDir)
}

// findgoversion determines the Go version to use in the version string.
func findgoversion() string {
	// The $GOROOT/VERSION file takes priority, for distributions
	// without the source repo.
	path := pathf("%s/VERSION", goroot)
	_p(1, "versionfile:", path)
	if isfile(path) {
		b := chomp(readfile(path))
		// Commands such as "dist version > VERSION" will cause
		// the shell to create an empty VERSION file and set dist's
		// stdout to its fd. dist in turn looks at VERSION and uses
		// its content if available, which is empty at this point.
		// Only use the VERSION file if it is non-empty.
		if b != "" {
			// Some builders cross-compile the toolchain on linux-amd64
			// and then copy the toolchain to the target builder (say, linux-arm)
			// for use there. But on non-release (devel) branches, the compiler
			// used on linux-amd64 will be an amd64 binary, and the compiler
			// shipped to linux-arm will be an arm binary, so they will have different
			// content IDs (they are binaries for different architectures) and so the
			// packages compiled by the running-on-amd64 compiler will appear
			// stale relative to the running-on-arm compiler. Avoid this by setting
			// the version string to something that doesn't begin with devel.
			// Then the version string will be used in place of the content ID,
			// and the packages will look up-to-date.
			// TODO(rsc): Really the builders could be writing out a better VERSION file instead,
			// but it is easier to change cmd/dist than to try to make changes to
			// the builder while Brad is away.
			if strings.HasPrefix(b, "devel") {
				if hostType := os.Getenv("META_BUILDLET_HOST_TYPE"); strings.Contains(hostType, "-cross") {
					fmt.Fprintf(os.Stderr, "warning: changing VERSION from %q to %q\n", b, "builder "+hostType)
					b = "builder " + hostType
				}
			}
			return b
		}
	}

	// The $GOROOT/VERSION.cache file is a cache to avoid invoking
	// git every time we run this command. Unlike VERSION, it gets
	// deleted by the clean command.
	path = pathf("%s/VERSION.cache", goroot)
	if isfile(path) {
		return chomp(readfile(path))
	}

	// Show a nicer error message if this isn't a Git repo.
	if !isGitRepo() {
		fatalf("FAILED: not a Git repo; must put a VERSION file in $GOROOT")
	}

	// Otherwise, use Git.
	// What is the current branch?
	branch := chomp(run(goroot, CheckExit, "git", "rev-parse", "--abbrev-ref", "HEAD"))

	// What are the tags along the current branch?
	tag := "devel"
	precise := false

	// If we're on a release branch, use the closest matching tag
	// that is on the release branch (and not on the master branch).
	if strings.HasPrefix(branch, "release-branch.") {
		tag, precise = branchtag(branch)
	}

	if !precise {
		// Tag does not point at HEAD; add hash and date to version.
		tag += chomp(run(goroot, CheckExit, "git", "log", "-n", "1", "--format=format: +%h %cd", "HEAD"))
	}

	// Cache version.
	writefile(tag, path, 0)

	return tag
}
