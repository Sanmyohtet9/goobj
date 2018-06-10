package main

import (
	"fmt"
	"os"

	"github.com/ks888/goobj"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("Usage: %s [goobj-file]\n", os.Args[0])
		os.Exit(1)
	}

	filename := os.Args[1]
	f, err := os.Open(filename)
	if err != nil {
		fmt.Printf("failed to open %s: %v\n", filename, err)
		os.Exit(1)
	}
	defer f.Close()

	parser := goobj.NewGoObjParser(f)
	if err = parser.Parse(); err != nil {
		fmt.Printf("failed to parse goobj file: %v\n", err)
		os.Exit(1)
	}
}