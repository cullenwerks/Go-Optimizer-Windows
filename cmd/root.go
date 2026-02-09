package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "syscleaner",
	Short: "SysCleaner - Windows System Cleaner & Gaming Optimizer",
	Long: `SysCleaner is a free, open-source Windows system optimizer for gamers.

Features:
  - Deep system cleaning (temp files, browser caches, logs)
  - Gaming mode (auto-detects games, boosts CPU/RAM priority)
  - Extreme mode (stops Explorer shell, maximum performance)
  - System optimizer (startup, network, disk optimizations)
  - CPU priority manager (permanent per-process priority settings)`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
