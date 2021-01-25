// Package internal implements an embedded filesystem for html static files
package internal

// Generate resource.go by [parcello](github.com/phogolabs/parcello)
//go:generate parcello -q -r -d ../../../html

import "github.com/phogolabs/parcello"

// FS returns embedded filesystem.
func FS() parcello.FileSystemManager {
	return parcello.Manager
}
