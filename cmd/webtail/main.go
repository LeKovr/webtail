//+build !test

// This file holds code which does not covered by tests

package main

import (
	"os"
)

func main() {
	Run(os.Exit)
}
