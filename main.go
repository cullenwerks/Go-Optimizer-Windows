//go:build gui

package main

import (
	"os"
	"syscleaner/cmd"
	"syscleaner/gui"
)

func main() {
	// If invoked as a background worker subprocess, run cobra instead of the GUI.
	for _, arg := range os.Args[1:] {
		if arg == "extreme-worker" {
			cmd.Execute()
			return
		}
	}
	gui.Run()
}
