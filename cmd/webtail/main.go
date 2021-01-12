//+build !test

// This file holds code which does not covered by tests

package main

import (
	"log"
	"os"
)

// Actual version value will be set at build time
var version = "0.0-dev"

// Actual build stamp value will be set at build time
var built = "now"

func main() {
	log.Printf("WebTail %s. Tail (log)files via web. Built at %s", version, built)
	run(os.Exit)
}
