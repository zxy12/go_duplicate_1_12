#!/usr/bin/env bash
# Copyright 2009 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# See golang.org/s/go15bootstrap for an overview of the build process.

# Environment variables that control make.bash:
#
# GOROOT_FINAL: The expected final Go root, baked into binaries.
# The default is the location of the Go tree during the build.
#
# GOHOSTARCH: The architecture for host tools (compilers and
# binaries).  Binaries of this type must be executable on the current
# system, so the only common reason to set this is to set
# GOHOSTARCH=386 on an amd64 machine.
#
# GOARCH: The target architecture for installed packages and tools.
#
# GOOS: The target operating system for installed packages and tools.
#
# GO_GCFLAGS: Additional go tool compile arguments to use when
# building the packages and commands.
#
# GO_LDFLAGS: Additional go tool link arguments to use when
# building the commands.
#
# CGO_ENABLED: Controls cgo usage during the build. Set it to 1
# to include all cgo related files, .c and .go file with "cgo"
# build directive, in the build. Set it to 0 to ignore them.
#
# GO_EXTLINK_ENABLED: Set to 1 to invoke the host linker when building
# packages that use cgo.  Set to 0 to do all linking internally.  This
# controls the default behavior of the linker's -linkmode option.  The
# default value depends on the system.
#
# CC: Command line to run to compile C code for GOHOSTARCH.
# Default is "gcc". Also supported: "clang".
#
# CC_FOR_TARGET: Command line to run to compile C code for GOARCH.
# This is used by cgo.  Default is CC.
#
# CXX_FOR_TARGET: Command line to run to compile C++ code for GOARCH.
# This is used by cgo. Default is CXX, or, if that is not set,
# "g++" or "clang++".
#
# FC: Command line to run to compile Fortran code for GOARCH.
# This is used by cgo. Default is "gfortran".
#
# PKG_CONFIG: Path to pkg-config tool. Default is "pkg-config".
#
# GO_DISTFLAGS: extra flags to provide to "dist bootstrap".
# (Or just pass them to the make.bash command line.)
#
# GOBUILDTIMELOGFILE: If set, make.bash and all.bash write
# timing information to this file. Useful for profiling where the
# time goes when these scripts run.
#
# GOROOT_BOOTSTRAP: A working Go tree >= Go 1.4 for bootstrap.
# If $GOROOT_BOOTSTRAP/bin/go is missing, $(go env GOROOT) is
# tried for all "go" in $PATH. $HOME/go1.4 by default.

set -e

unset GOBIN # Issue 14340
unset GOFLAGS
unset GO111MODULE


if [ ! -f run.bash ]; then
    echo 'make.bash must be run from $GOROOT/src' 1>&2
    exit 1
fi

if [ "$GOBUILDTIMELOGFILE" != "" ]; then
    echo $(LC_TIME=C date) start make.bash >"$GOBUILDTIMELOGFILE"
fi

# Test for Windows.
case "$(uname)" in
*MINGW* | *WIN32* | *CYGWIN*)
    echo 'ERROR: Do not use make.bash to build on Windows.'
    echo 'Use make.bat instead.'
    echo
    exit 1
    ;;
esac

# Test for bad ld.
if ld --version 2>&1 | grep 'gold.* 2\.20' >/dev/null; then
    echo 'ERROR: Your system has gold 2.20 installed.'
    echo 'This version is shipped by Ubuntu even though'
    echo 'it is known not to work on Ubuntu.'
    echo 'Binaries built with this linker are likely to fail in mysterious ways.'
    echo
    echo 'Run sudo apt-get remove binutils-gold.'
    echo
    exit 1
fi

# Test for bad SELinux.
# On Fedora 16 the selinux filesystem is mounted at /sys/fs/selinux,
# so loop through the possible selinux mount points.
for se_mount in /selinux /sys/fs/selinux
do
    if [ -d $se_mount -a -f $se_mount/booleans/allow_execstack -a -x /usr/sbin/selinuxenabled ] && /usr/sbin/selinuxenabled; then
        if ! cat $se_mount/booleans/allow_execstack | grep -c '^1 1$' >> /dev/null ; then
            echo "WARNING: the default SELinux policy on, at least, Fedora 12 breaks "
            echo "Go. You can enable the features that Go needs via the following "
            echo "command (as root):"
            echo "  # setsebool -P allow_execstack 1"
            echo
            echo "Note that this affects your system globally! "
            echo
            echo "The build will continue in five seconds in case we "
            echo "misdiagnosed the issue..."

            sleep 5
        fi
    fi
done

# Test for debian/kFreeBSD.
# cmd/dist will detect kFreeBSD as freebsd/$GOARCH, but we need to
# disable cgo manually.
if [ "$(uname -s)" = "GNU/kFreeBSD" ]; then
    export CGO_ENABLED=0
fi

# Clean old generated file that will cause problems in the build.
rm -f ./runtime/runtime_defs.go

# Finally!  Run the build.

verbose=false
vflag=""
if [ "$1" = "-v" ]; then
    verbose=true
    vflag=-v
    shift
fi

echo export GOROOT_BOOTSTRAP=${GOROOT_BOOTSTRAP:-$HOME/go1.4}
export GOROOT_BOOTSTRAP=${GOROOT_BOOTSTRAP:-$HOME/go1.4}
echo export GOROOT="$(cd .. && pwd)"
export GOROOT="$(cd .. && pwd)"
IFS=$'\n'; for go_exe in $(type -ap go); do
    if [ ! -x "$GOROOT_BOOTSTRAP/bin/go" ]; then
        goroot=$(GOROOT='' GOOS='' GOARCH='' "$go_exe" env GOROOT)
        if [ "$goroot" != "$GOROOT" ]; then
            GOROOT_BOOTSTRAP=$goroot
        fi
    fi
done; unset IFS
echo "Building Go cmd/dist using $GOROOT_BOOTSTRAP."

if $verbose; then
    echo cmd/dist
fi


if [ ! -x "$GOROOT_BOOTSTRAP/bin/go" ]; then
    echo "ERROR: Cannot find $GOROOT_BOOTSTRAP/bin/go." >&2
    echo "Set \$GOROOT_BOOTSTRAP to a working Go tree >= Go 1.4." >&2
    exit 1
fi
if [ "$GOROOT_BOOTSTRAP" = "$GOROOT" ]; then
    echo "ERROR: \$GOROOT_BOOTSTRAP must not be set to \$GOROOT" >&2
    echo "Set \$GOROOT_BOOTSTRAP to a working Go tree >= Go 1.4." >&2
    exit 1
fi
rm -f cmd/dist/dist
GOROOT="$GOROOT_BOOTSTRAP" GOOS="" GOARCH="" "$GOROOT_BOOTSTRAP/bin/go" build -o cmd/dist/dist ./cmd/dist