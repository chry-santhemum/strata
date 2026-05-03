package main

import (
	"os"

	"strata/internal/strata"
)

func main() {
	os.Exit(strata.Main(os.Args[1:], os.Stdout, os.Stderr))
}
