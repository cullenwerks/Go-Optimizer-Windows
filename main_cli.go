//go:build !gui

package main

import "syscleaner/cmd"

func main() {
	cmd.Execute()
}
