package main

import (
	"strings"
)

func clean() {

	for _, name := range cleanlist {
		path := pathf("%s/src/%s", goroot, name)

		// Remove generated files.
		for _, elem := range xreaddir(path) {
			for _, gt := range gentab {
				if strings.HasPrefix(elem, gt.nameprefix) {
					xremove(pathf("%s/%s", path, elem))
				}
			}
		}
		// Remove generated binary named for directory.
		if strings.HasPrefix(name, "cmd/") {
			xremove(pathf("%s/%s", path, name[4:]))
		}
	}

	// remove runtimegen files.
	path := pathf("%s/src/runtime", goroot)
	for _, elem := range runtimegen {
		xremove(pathf("%s/%s", path, elem))
	}

	if rebuildall {
		// Remove object tree.
		xremoveall(pathf("%s/pkg/obj/%s_%s", goroot, gohostos, gohostarch))

		// Remove installed packages and tools.
		xremoveall(pathf("%s/pkg/%s_%s", goroot, gohostos, gohostarch))
		xremoveall(pathf("%s/pkg/%s_%s", goroot, goos, goarch))
		xremoveall(pathf("%s/pkg/%s_%s_race", goroot, gohostos, gohostarch))
		xremoveall(pathf("%s/pkg/%s_%s_race", goroot, goos, goarch))
		xremoveall(tooldir)

		// Remove cached version info.
		xremove(pathf("%s/VERSION.cache", goroot))
	}
}
