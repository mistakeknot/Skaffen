package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		printVersion()
		return
	}

	fmt.Fprintln(os.Stderr, "skaffen: not yet implemented")
	os.Exit(1)
}

func printVersion() {
	version := "dev"
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		version = info.Main.Version
	}
	fmt.Printf("skaffen %s (%s)\n", version, runtime.Version())
}
