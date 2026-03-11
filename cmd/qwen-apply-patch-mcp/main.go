package main

import (
	"flag"
	"fmt"
	"os"

	"apply_patch_qwen/internal/adapters/mcp"
)

func main() {
	root := flag.String("root", ".", "workspace root")
	flag.Parse()

	server := mcp.New(*root)
	if err := server.Serve(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "mcp server: %v\n", err)
		os.Exit(1)
	}
}
