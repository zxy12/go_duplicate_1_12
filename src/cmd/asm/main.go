// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	//
	"cmd/asm/internal/arch"
	// "cmd/asm/internal/asm"
	"cmd/asm/internal/flags"
	"cmd/asm/internal/lex"
	//
	"cmd/internal/bio"
	"cmd/internal/obj"
	"cmd/internal/objabi"
)

func main() {
	log.SetFlags(log.LstdFlags)
	log.SetPrefix("asm: ")
	GOARCH := objabi.GOARCH
	log.Println(GOARCH)

	architecture := arch.Set(GOARCH)
	if architecture == nil {
		log.Fatalf("unrecognized architecture %s", GOARCH)
	}

	flags.Parse()

	ctxt := obj.Linknew(architecture.LinkArch)
	log.Printf("ctxt=%v", ctxt)

	if *flags.PrintOut {
		ctxt.Debugasm = 1
	}

	ctxt.Flag_dynlink = *flags.Dynlink
	ctxt.Flag_shared = *flags.Shared || *flags.Dynlink

	ctxt.Bso = bufio.NewWriter(os.Stdout)

	defer ctxt.Bso.Flush()

	architecture.Init(ctxt)

	// Create object file, write header.
	out, err := os.Create(*flags.OutputFile)
	if err != nil {
		log.Fatal(err)
	}

	defer bio.MustClose(out)

	buf := bufio.NewWriter(bio.MustWriter(out))

	if !*flags.SymABIs {
		fmt.Fprintf(buf, "go object %s %s %s\n", objabi.GOOS, objabi.GOARCH, objabi.Version)
		fmt.Fprintf(buf, "!\n")
	}

	//var ok, diag bool
	//var failedFile string

	for i, f := range flag.Args() {
		log.Println("arg-", i, f)
		lexer := lex.NewLexer(f)
		_ = lexer
		// parser := asm.NewParser(ctxt, architecture, lexer)
		// ctxt.DiagFunc = func(format string, args ...interface{}) {
		// 	diag = true
		// 	log.Printf(format, args...)
		// }
		// if *flags.SymABIs {
		// 	ok = parser.ParseSymABIs(buf)
		// } else {
		// 	pList := new(obj.Plist)
		// 	pList.Firstpc, ok = parser.Parse()
		// 	// reports errors to parser.Errorf
		// 	if ok {
		// 		obj.Flushplist(ctxt, pList, nil, "")
		// 	}
		// }
		// if !ok {
		// 	failedFile = f
		// 	break
		// }
		_ = f
	}

	/*
	   if ok && !*flags.SymABIs {
	       obj.WriteObjFile(ctxt, buf)
	   }
	   if !ok || diag {
	       if failedFile != "" {
	           log.Printf("assembly of %s failed", failedFile)
	       } else {
	           log.Print("assembly failed")
	       }
	       out.Close()
	       os.Remove(*flags.OutputFile)
	       os.Exit(1)
	   }
	*/
	buf.Flush()

}
