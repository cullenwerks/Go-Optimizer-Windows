//go:build windows

package gaming

import (
	"bufio"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// RunExtremeModeWorker spawns the current executable as a hidden child process
// with the "--extreme-worker <action>" flag. Progress lines written to the child's
// stdout are forwarded to the progress callback. Blocks until the child exits.
//
// action must be "enable" or "disable".
func RunExtremeModeWorker(action string, progress func(string)) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to locate executable: %w", err)
	}

	// Build command line: `"<exe>" extreme-worker <action>`
	cmdLine := fmt.Sprintf(`"%s" extreme-worker %s`, exePath, action)
	cmdLinePtr, err := windows.UTF16PtrFromString(cmdLine)
	if err != nil {
		return fmt.Errorf("failed to encode command line: %w", err)
	}

	// Create a pipe for the child's stdout.
	var readPipe, writePipe windows.Handle
	sa := windows.SecurityAttributes{InheritHandle: 1}
	sa.Length = uint32(unsafe.Sizeof(sa))
	if err := windows.CreatePipe(&readPipe, &writePipe, &sa, 0); err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	// NOTE: do NOT defer CloseHandle(readPipe) here -- os.NewFile below
	// takes ownership and installs a finalizer; explicit Close() below handles it.

	// Mark the read end as non-inheritable so the child doesn't get a copy of it.
	windows.SetHandleInformation(readPipe, windows.HANDLE_FLAG_INHERIT, 0)

	si := windows.StartupInfo{
		Flags:      windows.STARTF_USESTDHANDLES | windows.STARTF_USESHOWWINDOW,
		ShowWindow: windows.SW_HIDE,
		StdOutput:  writePipe,
		StdErr:     writePipe,
	}
	si.Cb = uint32(unsafe.Sizeof(si))

	var pi windows.ProcessInformation

	// CREATE_NO_WINDOW prevents a console window from flashing.
	const createNoWindow = 0x08000000
	err = windows.CreateProcess(
		nil,
		cmdLinePtr,
		nil,
		nil,
		true, // inherit handles so child gets writePipe as stdout
		createNoWindow,
		nil,
		nil,
		&si,
		&pi,
	)
	// Close the write end in the parent — child now owns it exclusively.
	windows.CloseHandle(writePipe)
	if err != nil {
		windows.CloseHandle(readPipe)
		return fmt.Errorf("failed to spawn worker process: %w", err)
	}
	defer windows.CloseHandle(pi.Thread)
	defer windows.CloseHandle(pi.Process)

	// Read progress lines from the child's stdout pipe.
	reader := os.NewFile(uintptr(readPipe), "pipe")
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if progress != nil && line != "" {
			progress(line)
		}
	}
	reader.Close() // closes readPipe and clears the GC finalizer

	// Wait for the child to exit completely.
	windows.WaitForSingleObject(pi.Process, windows.INFINITE)

	var exitCode uint32
	if err := windows.GetExitCodeProcess(pi.Process, &exitCode); err != nil {
		return fmt.Errorf("failed to get worker exit code: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("extreme mode worker exited with code %d", exitCode)
	}
	return nil
}
