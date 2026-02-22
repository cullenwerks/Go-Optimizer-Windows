//go:build windows

package gaming

import (
	"fmt"
	"log"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	user32           = windows.NewLazySystemDLL("user32.dll")
	procFindWindowW  = user32.NewProc("FindWindowW")
	procPostMessageW = user32.NewProc("PostMessageW")
)

const wmClose = 0x0010

// terminateProcessByName finds and terminates a process by its executable name
// using native Windows APIs instead of spawning taskkill.exe child processes.
// This avoids triggering AV heuristics from rapid child process spawning.
func terminateProcessByName(name string) error {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return fmt.Errorf("failed to create process snapshot: %w", err)
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	err = windows.Process32First(snapshot, &entry)
	if err != nil {
		return fmt.Errorf("failed to enumerate processes: %w", err)
	}

	nameLower := strings.ToLower(name)
	terminated := false

	for {
		exeName := windows.UTF16ToString(entry.ExeFile[:])
		if strings.ToLower(exeName) == nameLower {
			handle, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, entry.ProcessID)
			if err != nil {
				// Process may have exited or we lack permissions; skip
			} else {
				if err := windows.TerminateProcess(handle, 0); err == nil {
					terminated = true
				}
				windows.CloseHandle(handle)
			}
		}

		err = windows.Process32Next(snapshot, &entry)
		if err != nil {
			break
		}
	}

	if !terminated {
		return fmt.Errorf("process %s not found or could not be terminated", name)
	}
	return nil
}

const explorerAutoRestartKey = `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Winlogon`

// stopWindowsExplorerNative disables the Session Manager auto-restart of explorer.exe,
// then terminates it via TerminateProcess. Setting AutoRestartShell=0 prevents
// smss.exe from relaunching explorer after a forced kill, which is the only
// reliable method on Windows 11 (WM_CLOSE to Shell_TrayWnd opens the shutdown
// dialog on Windows 11 rather than cleanly exiting the shell).
func stopWindowsExplorerNative() error {
	// Disable Session Manager auto-restart so TerminateProcess won't trigger a relaunch.
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, explorerAutoRestartKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open Winlogon key: %w", err)
	}
	if err := key.SetDWordValue("AutoRestartShell", 0); err != nil {
		key.Close()
		return fmt.Errorf("failed to set AutoRestartShell=0: %w", err)
	}
	key.Close()

	// Now terminate explorer.exe.
	if err := terminateProcessByName("explorer.exe"); err != nil {
		// Restore auto-restart before returning the error.
		if k, e := registry.OpenKey(registry.LOCAL_MACHINE, explorerAutoRestartKey, registry.SET_VALUE); e == nil {
			_ = k.SetDWordValue("AutoRestartShell", 1)
			k.Close()
		}
		return fmt.Errorf("failed to terminate explorer.exe: %w", err)
	}

	// Poll until explorer is fully gone (up to 5 seconds).
	className, _ := windows.UTF16PtrFromString("Shell_TrayWnd")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
		check, _, _ := procFindWindowW.Call(uintptr(unsafe.Pointer(className)), 0)
		if check == 0 {
			return nil // Shell is gone
		}
	}
	log.Println("[SysCleaner] Explorer process terminated but Shell_TrayWnd still present; proceeding anyway")
	return nil
}
