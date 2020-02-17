package main

import (
	"encoding/json"
	"flag"
	"os"
	"sort"
	"strings"
)

// cmdlist lists all supported platforms.
func _cmdlist() {
	jsonFlag := flag.Bool("json", false, "produce JSON output")
	xflagparse(0)

	var plats []string
	for p := range cgoEnabled {
		if incomplete[p] {
			continue
		}
		plats = append(plats, p)
	}
	sort.Strings(plats)

	if !*jsonFlag {
		for _, p := range plats {
			xprintf("%s\n", p)
		}
		return
	}

	type jsonResult struct {
		GOOS         string
		GOARCH       string
		CgoSupported bool
	}
	var results []jsonResult
	for _, p := range plats {
		fields := strings.Split(p, "/")
		results = append(results, jsonResult{
			GOOS:         fields[0],
			GOARCH:       fields[1],
			CgoSupported: cgoEnabled[p]})
	}
	out, err := json.MarshalIndent(results, "", "\t")
	if err != nil {
		fatalf("json marshal error: %v", err)
	}
	//xprintf(string(out))
	if _, err := os.Stdout.Write(out); err != nil {
		fatalf("write failed: %v", err)
	}
}
