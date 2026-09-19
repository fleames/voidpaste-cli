// Command voidpaste is the VoidPaste CLI for https://voidpaste.com.
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := root(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
