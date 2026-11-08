package main

import (
	"flag"
	"fmt"
	"os"

	"adaptive-ai-gateway/internal/architecture"
)

func main() {
	root := flag.String("root", ".", "repository root")
	manifest := flag.String("manifest", "architecture/dependencies.json", "architecture manifest")
	flag.Parse()
	violations, err := architecture.Check(*root, *manifest)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, violation := range violations {
		fmt.Fprintln(os.Stderr, violation.String())
	}
	if len(violations) != 0 {
		os.Exit(1)
	}
	fmt.Println("architecture dependencies: passed")
}
