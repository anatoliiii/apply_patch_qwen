package main

import (
	"flag"
	"fmt"
	"os"

	"apply_patch_qwen/internal/adapters/discovery"
)

func main() {
	root := flag.String("root", ".", "workspace root")
	flag.Parse()

	if flag.NArg() < 1 {
		fatalf("usage: qwen-apply-patch-tool [-root PATH] <discovery|call TOOL|TOOL>")
	}
	switch flag.Arg(0) {
	case "discovery":
		if err := discovery.WriteDocument(os.Stdout); err != nil {
			fatalf("discovery: %v", err)
		}
	case "call":
		if flag.NArg() < 2 {
			fatalf("call mode requires a tool name")
		}
		if err := discovery.Execute(*root, flag.Arg(1), os.Stdin, os.Stdout); err != nil {
			fatalf("call: %v", err)
		}
	default:
		if err := discovery.Execute(*root, flag.Arg(0), os.Stdin, os.Stdout); err != nil {
			fatalf("call: %v", err)
		}
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
