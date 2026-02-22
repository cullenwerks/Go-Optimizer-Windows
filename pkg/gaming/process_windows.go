//go:build windows

package gaming

import (
	"fmt"
	"log"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32           = windows.NewLazySystemDLL("user32.dll")
	procFindWindowW  = user32.NewProc("FindWindowW")
	procPostMessageW = user32.NewProc("PostMessageW")
)

const wmQuit = 0x0012

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

// stopWindowsExplorerNative sends WM_QUIT to the Shell_TrayWnd (taskbar window),
// which causes explorer.exe to shut down cleanly. Unlike TerminateProcess, a clean
// quit does NOT trigger the Session Manager to auto-restart explorer.
// Polls up to 5 seconds for confirmation, then falls back to TerminateProcess.
func stopWindowsExplorerNative() error {
	className, _ := windows.UTF16PtrFromString("Shell_TrayWnd")

	hwnd, _, _ := procFindWindowW.Call(
		uintptr(unsafe.Pointer(className)),
		0,
	)

	if hwnd == 0 {
		// Explorer is not running — nothing to stop
		return nil
	}

	// Post WM_QUIT to the shell window
	procPostMessageW.Call(hwnd, wmQuit, 0, 0)

	// Poll until explorer is gone (up to 5 seconds)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
		check, _, _ := procFindWindowW.Call(
			uintptr(unsafe.Pointer(className)),
			0,
		)
		if check == 0 {
			return nil // Shell_TrayWnd gone — explorer stopped
		}
	}

	// Fallback: force-terminate if WM_QUIT didn't work
	log.Println("[SysCleaner] WM_QUIT timeout, falling back to TerminateProcess for explorer.exe")
	return terminateProcessByName("explorer.exe")
}
