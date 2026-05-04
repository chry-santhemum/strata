package main

import (
	"os"

	"strata/internal/strata"
)

func main() {
	os.Exit(strata.Main(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
