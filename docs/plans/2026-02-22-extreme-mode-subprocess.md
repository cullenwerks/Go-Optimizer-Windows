# Extreme Mode Subprocess Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Run the extreme mode activation/deactivation logic in a child subprocess so the GUI process stays alive and responsive when Windows Explorer is killed.

**Architecture:** The GUI binary re-launches itself with a hidden internal flag (`--extreme-worker enable|disable`) to perform the privileged work in a separate process. The parent GUI reads the child's stdout line-by-line for progress updates and waits on the child process handle to detect completion. The user just clicks the button — no extra steps.

**Key insight:** The same compiled binary serves as both the GUI and the worker. When launched with `--extreme-worker`, it runs headless, does the work, prints progress to stdout, and exits. The GUI spawns it via `CreateProcess` with a stdout pipe, reads progress lines, and updates the button text in real time.

**Tech Stack:** Go, Windows `CreateProcess` / `ReadFile` APIs via `golang.org/x/sys/windows`, Fyne GUI, existing `gaming.EnableExtremeMode` / `gaming.DisableExtremeMode` functions.

---

## Background: Why This Approach

When `stopWindowsExplorerNative()` kills `explorer.exe`, the Win32 desktop shell that Fyne's OpenGL/GLFW window depends on becomes disrupted. The GUI process's message queue stalls. Running the work in a goroutine inside the GUI process doesn't help — the problem is OS-level, not goroutine-level.

Solution: the child process does the work (including killing explorer). The parent GUI process never touches explorer, so its window stays live.

---

## Task 1: Add the `--extreme-worker` internal sub-command

**Files:**
- Modify: `cmd/extreme.go`
- Modify: `cmd/root.go` (check if `--extreme-worker` is present before building GUI — see Task 2)

The worker sub-command is hidden (not shown in `--help`). It reads `enable` or `disable` from its argument, calls the existing `gaming.Enable/DisableExtremeMode`, prints progress lines to stdout, and exits.

**Step 1: Add the hidden worker command to `cmd/extreme.go`**

Add this block at the bottom of `cmd/extreme.go`, inside the `init()` function (after existing flag registrations):

```go
var extremeWorkerCmd = &cobra.Command{
    Use:    "--extreme-worker",
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
```

In `init()`, add:
```go
rootCmd.AddCommand(extremeWorkerCmd)
```

Also add `"os"` to the import list in `cmd/extreme.go`.

**Step 2: Build and verify the command exists**

```bash
go build -tags gui -o SysCleaner-test.exe && ./SysCleaner-test.exe --help
```

Expected: `--extreme-worker` does NOT appear in help output (it's hidden).

**Step 3: Manually test the worker (temporary — remove after)**

```bash
./SysCleaner-test.exe --extreme-worker enable
```

Expected: progress lines printed, then `DONE` or an error if not running as admin. (This is just a smoke test — don't worry about actually activating extreme mode here.)

**Step 4: Commit**

```bash
git add cmd/extreme.go
git commit -m "feat: add hidden --extreme-worker sub-command for subprocess execution"
```

---

## Task 2: Create the subprocess launcher (Windows-only)

**Files:**
- Create: `pkg/gaming/worker_windows.go`

This file contains the function the GUI will call instead of `gaming.EnableExtremeMode` directly. It:
1. Finds the path to the running executable (`os.Executable()`)
2. Spawns itself as a child process with `--extreme-worker enable` (or `disable`)
3. Creates a stdout pipe to read progress lines
4. Reads lines in a loop, calls `progress(line)` for each
5. Waits for the child to exit
6. Returns an error if exit code != 0

**Step 1: Create `pkg/gaming/worker_windows.go`**

```go
//go:build windows

package gaming

import (
    "bufio"
    "fmt"
    "io"
    "os"
    "syscall"
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

    // Build command line: `"<exe>" --extreme-worker <action>`
    cmdLine := fmt.Sprintf(`"%s" --extreme-worker %s`, exePath, action)
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
    // The write end is inherited by the child; close it in the parent after CreateProcess.
    defer windows.CloseHandle(readPipe)

    // Mark the read end as non-inheritable so the child doesn't get a copy.
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
        true, // inherit handles (so child gets writePipe as stdout)
        createNoWindow,
        nil,
        nil,
        &si,
        &pi,
    )
    // Close the write end of the pipe in the parent — child now owns it.
    windows.CloseHandle(writePipe)
    if err != nil {
        return fmt.Errorf("failed to spawn worker process: %w", err)
    }
    defer windows.CloseHandle(pi.Thread)
    defer windows.CloseHandle(pi.Process)

    // Read progress lines from the child's stdout.
    reader := io.Reader(os.NewFile(uintptr(readPipe), "pipe"))
    scanner := bufio.NewScanner(reader)
    for scanner.Scan() {
        line := scanner.Text()
        if progress != nil && line != "" {
            progress(line)
        }
    }

    // Wait for the child to exit.
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
```

**Step 2: Create the stub for non-Windows builds**

Create `pkg/gaming/worker_other.go`:

```go
//go:build !windows

package gaming

import "fmt"

func RunExtremeModeWorker(action string, progress func(string)) error {
    return fmt.Errorf("extreme mode subprocess only supported on Windows")
}
```

**Step 3: Build to confirm it compiles**

```bash
go build -tags gui ./...
```

Expected: no errors.

**Step 4: Commit**

```bash
git add pkg/gaming/worker_windows.go pkg/gaming/worker_other.go
git commit -m "feat: add RunExtremeModeWorker to spawn extreme mode as child process"
```

---

## Task 3: Wire the GUI to use the subprocess launcher

**Files:**
- Modify: `gui/views/extreme_mode.go`

Replace the two direct calls to `gaming.EnableExtremeMode` and `gaming.DisableExtremeMode` with calls to `gaming.RunExtremeModeWorker("enable", ...)` and `gaming.RunExtremeModeWorker("disable", ...)`.

**Step 1: Update `toggleExtremeMode` in `gui/views/extreme_mode.go`**

Find the activation goroutine (lines ~119-130):

```go
go func() {
    defer p.toggleBtn.Enable()
    if err := gaming.EnableExtremeMode(func(msg string) {
        p.toggleBtn.SetText(msg)
    }); err != nil {
        dialog.ShowError(err, p.window)
        return
    }
    p.isActive = true
    dialog.ShowInformation("Extreme Mode Activated", "System optimized for maximum performance!", p.window)
    p.updateUI()
}()
```

Replace `gaming.EnableExtremeMode(...)` with `gaming.RunExtremeModeWorker("enable", ...)`:

```go
go func() {
    defer p.toggleBtn.Enable()
    if err := gaming.RunExtremeModeWorker("enable", func(msg string) {
        p.toggleBtn.SetText(msg)
    }); err != nil {
        dialog.ShowError(err, p.window)
        return
    }
    p.isActive = true
    dialog.ShowInformation("Extreme Mode Activated", "System optimized for maximum performance!", p.window)
    p.updateUI()
}()
```

Find the deactivation goroutine (lines ~94-105):

```go
go func() {
    defer p.toggleBtn.Enable()
    if err := gaming.DisableExtremeMode(func(msg string) {
        p.toggleBtn.SetText(msg)
    }); err != nil {
        dialog.ShowError(err, p.window)
        return
    }
    p.isActive = false
    dialog.ShowInformation("Extreme Mode Disabled", "System restored to normal mode.", p.window)
    p.updateUI()
}()
```

Replace `gaming.DisableExtremeMode(...)` with `gaming.RunExtremeModeWorker("disable", ...)`:

```go
go func() {
    defer p.toggleBtn.Enable()
    if err := gaming.RunExtremeModeWorker("disable", func(msg string) {
        p.toggleBtn.SetText(msg)
    }); err != nil {
        dialog.ShowError(err, p.window)
        return
    }
    p.isActive = false
    dialog.ShowInformation("Extreme Mode Disabled", "System restored to normal mode.", p.window)
    p.updateUI()
}()
```

**Step 2: Build to confirm it compiles**

```bash
go build -tags gui ./...
```

Expected: no errors.

**Step 3: Commit**

```bash
git add gui/views/extreme_mode.go
git commit -m "feat: GUI uses subprocess worker for extreme mode instead of in-process call"
```

---

## Task 4: Handle the worker's state detection correctly

**Context:** `gaming.IsExtremeModeActive()` reads an in-process variable. After switching to subprocess, the parent GUI process never calls `EnableExtremeMode` directly, so `extremeModeActive` in the parent is always `false`. The status polling goroutine in `extreme_mode.go` uses this to update UI.

We need a way for the parent to know if extreme mode is actually active. The simplest approach: write a sentinel file when active, delete it when inactive. The child process writes/deletes it; the parent polls for it.

**Files:**
- Modify: `pkg/gaming/extreme.go` — write/delete sentinel at start/end of Enable/Disable
- Modify: `pkg/gaming/extreme.go` — update `IsExtremeModeActive()` to check sentinel file on Windows when the in-process flag is false

**Step 1: Add sentinel file helpers to `pkg/gaming/extreme.go`**

Add these constants and helpers near the top of the file (after the `var` block):

```go
import "path/filepath"

// extremeSentinelPath returns the path to the sentinel file that signals
// extreme mode is active. Using %TEMP% ensures it's writable by admin processes
// and cleaned up on reboot.
func extremeSentinelPath() string {
    return filepath.Join(os.TempDir(), "syscleaner_extreme_active")
}

func writeSentinel() {
    f, err := os.Create(extremeSentinelPath())
    if err == nil {
        f.Close()
    }
}

func deleteSentinel() {
    os.Remove(extremeSentinelPath())
}

func sentinelExists() bool {
    _, err := os.Stat(extremeSentinelPath())
    return err == nil
}
```

**Step 2: Call `writeSentinel()` at end of `EnableExtremeMode`, `deleteSentinel()` at end of `DisableExtremeMode`**

In `EnableExtremeMode`, just before `return nil` at line ~314:
```go
writeSentinel()
```

In `DisableExtremeMode`, just before `return err` at line ~368:
```go
deleteSentinel()
```

**Step 3: Update `IsExtremeModeActive()` to check sentinel**

```go
func IsExtremeModeActive() bool {
    mu.Lock()
    defer mu.Unlock()
    if extremeModeActive {
        return true
    }
    // When running as the GUI parent, the in-process flag is always false
    // because Enable was called in a child process. Fall back to sentinel file.
    return sentinelExists()
}
```

**Step 4: Make sure `os` is imported in `extreme.go`** (it should already be via `log`, but verify)

**Step 5: Build**

```bash
go build -tags gui ./...
```

Expected: no errors.

**Step 6: Commit**

```bash
git add pkg/gaming/extreme.go
git commit -m "feat: use sentinel file so GUI parent can detect extreme mode state from child process"
```

---

## Task 5: Verify end-to-end (manual test)

**Step 1: Build a debug binary (with console output)**

```bash
go build -tags gui -o SysCleaner-debug.exe
```

**Step 2: Run as administrator**

Right-click `SysCleaner-debug.exe` → Run as administrator.

**Step 3: Navigate to Extreme Mode tab, click "ACTIVATE EXTREME PERFORMANCE MODE", confirm**

Expected:
- Button text updates with progress messages ("Closing background apps...", "Stopping non-essential services...", etc.)
- GUI **does not freeze** — the window remains interactive during the process
- After completion, a success dialog appears
- Status label shows "EXTREME MODE ACTIVE"

**Step 4: Click "Disable Extreme Mode & Restore System"**

Expected:
- Button text updates with progress
- GUI stays responsive
- Explorer restarts, taskbar reappears
- Status label returns to "Extreme Mode: Inactive"

**Step 5: Delete debug binary**

```bash
rm SysCleaner-debug.exe
```

---

## Task 6: Clean up and final build

**Step 1: Remove the temporary test binary if still present**

```bash
rm SysCleaner-test.exe 2>/dev/null || true
```

**Step 2: Run existing tests**

```bash
go test ./...
```

Expected: all tests pass.

**Step 3: Build release binary**

```powershell
./build.ps1
```

Expected: `SysCleaner-x64.exe` created successfully.

**Step 4: Final commit**

```bash
git add -A
git commit -m "chore: final build verification for extreme mode subprocess"
```

---

## Notes for Implementer

- The `--extreme-worker` flag is entirely internal — the user never sees or types it. The binary detects it automatically.
- The build tag is `gui`. All files in `gui/` and the `main.go` use `//go:build gui`. The worker command in `cmd/extreme.go` has no build tag, so it's compiled into both GUI and non-GUI builds — that's intentional and correct.
- `RunExtremeModeWorker` blocks until the child exits, but it's always called from a goroutine in the GUI, so the UI stays responsive.
- The sentinel file lives in `%TEMP%` and is automatically cleaned up on reboot — no registry entries, no permanent side effects.
- If the child process crashes mid-activation (e.g. power loss), the sentinel file may be left behind. On next launch, `IsExtremeModeActive()` will return true. The "Disable" path in `DisableExtremeMode` is safe to call in this state — it will restore explorer and services as best it can, then delete the sentinel.
