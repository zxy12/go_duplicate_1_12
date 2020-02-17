// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package arch defines architecture-specific information and support functions.
package arch

import (
	"cmd/internal/obj"
	//"cmd/internal/obj/arm"
	//"cmd/internal/obj/arm64"
	//"cmd/internal/obj/mips"
	//"cmd/internal/obj/ppc64"
	//"cmd/internal/obj/s390x"
	//"cmd/internal/obj/wasm"
	"cmd/internal/obj/x86"
	//"fmt"
	"log"
	"strings"
)

// Pseudo-registers whose names are the constant name without the leading R.
const (
	RFP = -(iota + 1)
	RSB
	RSP
	RPC
)

// Arch wraps the link architecture object with more architecture-specific information.
type Arch struct {
	*obj.LinkArch
	// Map of instruction names to enumeration.
	Instructions map[string]obj.As
	// Map of register names to enumeration.
	Register map[string]int16
	// Table of register prefix names. These are things like R for R(0) and SPR for SPR(268).
	RegisterPrefix map[string]bool
	// RegisterNumber converts R(10) into arm.REG_R10.
	RegisterNumber func(string, int16) (int16, bool)
	// Instruction is a jump.
	IsJump func(word string) bool
}

// nilRegisterNumber is the register number function for architectures
// that do not accept the R(N) notation. It always returns failure.
func nilRegisterNumber(name string, n int16) (int16, bool) {
	return 0, false
}

func jumpX86(word string) bool {
	return word[0] == 'J' || word == "CALL" || strings.HasPrefix(word, "LOOP") || word == "XBEGIN"
}

// Set configures the architecture specified by GOARCH and returns its representation.
// It returns nil if GOARCH is not recognized.
func Set(GOARCH string) *Arch {
	log.Printf("Arch set GOARCH[%s]\n", GOARCH)
	switch GOARCH {
	case "amd64":
		return archX86(&x86.Linkamd64)
	}

	return nil
}

func archX86(linkArch *obj.LinkArch) *Arch {
	register := make(map[string]int16)
	// Create maps for easy lookup of instruction names etc.
	for i, s := range x86.Register {
		register[s] = int16(i + x86.REG_AL)
	}
	// Pseudo-registers.
	register["SB"] = RSB
	register["FP"] = RFP
	register["PC"] = RPC
	// Register prefix not used on this architecture.

	instructions := make(map[string]obj.As)
	for i, s := range obj.Anames {
		instructions[s] = obj.As(i)
	}
	for i, s := range x86.Anames {
		if obj.As(i) >= obj.A_ARCHSPECIFIC {
			instructions[s] = obj.As(i) + obj.ABaseAMD64
		}
	}
	// Annoying aliases.
	instructions["JA"] = x86.AJHI   /* alternate */
	instructions["JAE"] = x86.AJCC  /* alternate */
	instructions["JB"] = x86.AJCS   /* alternate */
	instructions["JBE"] = x86.AJLS  /* alternate */
	instructions["JC"] = x86.AJCS   /* alternate */
	instructions["JCC"] = x86.AJCC  /* carry clear (CF = 0) */
	instructions["JCS"] = x86.AJCS  /* carry set (CF = 1) */
	instructions["JE"] = x86.AJEQ   /* alternate */
	instructions["JEQ"] = x86.AJEQ  /* equal (ZF = 1) */
	instructions["JG"] = x86.AJGT   /* alternate */
	instructions["JGE"] = x86.AJGE  /* greater than or equal (signed) (SF = OF) */
	instructions["JGT"] = x86.AJGT  /* greater than (signed) (ZF = 0 && SF = OF) */
	instructions["JHI"] = x86.AJHI  /* higher (unsigned) (CF = 0 && ZF = 0) */
	instructions["JHS"] = x86.AJCC  /* alternate */
	instructions["JL"] = x86.AJLT   /* alternate */
	instructions["JLE"] = x86.AJLE  /* less than or equal (signed) (ZF = 1 || SF != OF) */
	instructions["JLO"] = x86.AJCS  /* alternate */
	instructions["JLS"] = x86.AJLS  /* lower or same (unsigned) (CF = 1 || ZF = 1) */
	instructions["JLT"] = x86.AJLT  /* less than (signed) (SF != OF) */
	instructions["JMI"] = x86.AJMI  /* negative (minus) (SF = 1) */
	instructions["JNA"] = x86.AJLS  /* alternate */
	instructions["JNAE"] = x86.AJCS /* alternate */
	instructions["JNB"] = x86.AJCC  /* alternate */
	instructions["JNBE"] = x86.AJHI /* alternate */
	instructions["JNC"] = x86.AJCC  /* alternate */
	instructions["JNE"] = x86.AJNE  /* not equal (ZF = 0) */
	instructions["JNG"] = x86.AJLE  /* alternate */
	instructions["JNGE"] = x86.AJLT /* alternate */
	instructions["JNL"] = x86.AJGE  /* alternate */
	instructions["JNLE"] = x86.AJGT /* alternate */
	instructions["JNO"] = x86.AJOC  /* alternate */
	instructions["JNP"] = x86.AJPC  /* alternate */
	instructions["JNS"] = x86.AJPL  /* alternate */
	instructions["JNZ"] = x86.AJNE  /* alternate */
	instructions["JO"] = x86.AJOS   /* alternate */
	instructions["JOC"] = x86.AJOC  /* overflow clear (OF = 0) */
	instructions["JOS"] = x86.AJOS  /* overflow set (OF = 1) */
	instructions["JP"] = x86.AJPS   /* alternate */
	instructions["JPC"] = x86.AJPC  /* parity clear (PF = 0) */
	instructions["JPE"] = x86.AJPS  /* alternate */
	instructions["JPL"] = x86.AJPL  /* non-negative (plus) (SF = 0) */
	instructions["JPO"] = x86.AJPC  /* alternate */
	instructions["JPS"] = x86.AJPS  /* parity set (PF = 1) */
	instructions["JS"] = x86.AJMI   /* alternate */
	instructions["JZ"] = x86.AJEQ   /* alternate */
	instructions["MASKMOVDQU"] = x86.AMASKMOVOU
	instructions["MOVD"] = x86.AMOVQ
	instructions["MOVDQ2Q"] = x86.AMOVQ
	instructions["MOVNTDQ"] = x86.AMOVNTO
	instructions["MOVOA"] = x86.AMOVO
	instructions["PSLLDQ"] = x86.APSLLO
	instructions["PSRLDQ"] = x86.APSRLO
	instructions["PADDD"] = x86.APADDL

	return &Arch{
		LinkArch:       linkArch,
		Instructions:   instructions,
		Register:       register,
		RegisterPrefix: nil,
		RegisterNumber: nilRegisterNumber,
		IsJump:         jumpX86,
	}
}
