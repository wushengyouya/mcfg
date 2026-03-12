package main

import (
	"os"

	"mcfg/cmd"
)

func main() {
	os.Exit(cmd.Execute(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
