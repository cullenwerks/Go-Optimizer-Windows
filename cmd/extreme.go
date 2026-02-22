package cmd

import (
	"fmt"
	"os"

	"syscleaner/pkg/gaming"

	"github.com/spf13/cobra"
)

var extremeCmd = &cobra.Command{
	Use:   "extreme",
	Short: "Extreme performance mode - maximum system optimization",
	Long: `Extreme performance mode stops Windows Explorer and all non-essential services
for maximum gaming performance. Anti-cheat services are preserved.

WARNING: This mode removes the desktop shell. Use the GUI launcher to start games.`,
	Run: func(cmd *cobra.Command, args []string) {
		// All flags below are registered in init(), so GetBool errors are impossible.
		enable, _ := cmd.Flags().GetBool("enable")
		disable, _ := cmd.Flags().GetBool("disable")
		showStatus, _ := cmd.Flags().GetBool("status")

		if enable {
			fmt.Println("Enabling extreme performance mode...")
			fmt.Println()
			fmt.Println("WARNING: This will stop Windows Explorer (no desktop/taskbar).")
			fmt.Println("Use 'syscleaner extreme --disable' to restore.")
			fmt.Println()

			if err := gaming.EnableExtremeMode(nil); err != nil {
				fmt.Printf("  Error: %v\n", err)
				return
			}

			fmt.Println("  Stopped Windows Explorer")
			fmt.Println("  Stopped all non-essential services")
			fmt.Println("  Enabled anti-cheat services")
			fmt.Println("  Set ultimate performance power plan")
			fmt.Println("  Disabled visual effects")
			fmt.Println()
			fmt.Println("EXTREME PERFORMANCE MODE is now ACTIVE")
		} else if disable {
			fmt.Println("Disabling extreme performance mode...")
			fmt.Println()

			if err := gaming.DisableExtremeMode(nil); err != nil {
				fmt.Printf("  Error: %v\n", err)
				return
			}

			fmt.Println("  Restored Windows Explorer")
			fmt.Println("  Restored services")
			fmt.Println("  Re-enabled visual effects")
			fmt.Println("  Restored balanced power plan")
			fmt.Println()
			fmt.Println("System restored to normal mode.")
		} else if showStatus {
			printExtremeStatus()
		} else {
			printExtremeStatus()
		}
	},
}

var extremeWorkerCmd = &cobra.Command{
	Use:    "extreme-worker",
	Hidden: true,
	Args:   cobra.ExactArgs(1), // "enable" or "disable"
	Run: func(cmd *cobra.Command, args []string) {
		action := args[0]
		progress := func(msg string) {
			fmt.Println(msg)
		}
		switch action {
		case "enable":
			if err := gaming.EnableExtremeMode(progress); err != nil {
				fmt.Fprintln(os.Stderr, "ERROR:", err)
				os.Exit(1)
			}
			fmt.Println("DONE")
		case "disable":
			if err := gaming.DisableExtremeMode(progress); err != nil {
				fmt.Fprintln(os.Stderr, "ERROR:", err)
				os.Exit(1)
			}
			fmt.Println("DONE")
		default:
			fmt.Fprintln(os.Stderr, "unknown action:", action)
			os.Exit(1)
		}
	},
}

func printExtremeStatus() {
	fmt.Println("--- Extreme Performance Mode Status ---")
	fmt.Println()

	if gaming.IsExtremeModeActive() {
		fmt.Println("  Status:  ACTIVE")
		fmt.Println()
		fmt.Println("  Windows Explorer: STOPPED")
		fmt.Println("  Visual Effects:   DISABLED")
		fmt.Println("  Power Plan:       Ultimate Performance")
	} else {
		fmt.Println("  Status:  INACTIVE")
	}
	fmt.Println()

	if gaming.IsEnabled() {
		fmt.Println("  Gaming Mode:      ACTIVE")
	} else {
		fmt.Println("  Gaming Mode:      INACTIVE")
	}
}

func init() {
	extremeCmd.Flags().Bool("enable", false, "Enable extreme performance mode")
	extremeCmd.Flags().Bool("disable", false, "Disable extreme performance mode")
	extremeCmd.Flags().Bool("status", false, "Show extreme mode status")
	rootCmd.AddCommand(extremeCmd)
	rootCmd.AddCommand(extremeWorkerCmd)
}
