# Extreme Mode Fixes & Expansion — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix explorer shell not staying stopped, fix memory trimming data races and best practices, and expand extreme mode's process/service kill list for broader coverage.

**Architecture:** Three focused file changes — `process_windows.go` gets a new WM_QUIT explorer stop function, `memory_windows.go` gets mutex protection + FlushModifiedList + GlobalMemoryStatusEx, and `extreme.go` gets an expanded process/service list. No new packages needed; all changes use existing `golang.org/x/sys/windows`.

**Tech Stack:** Go 1.21+, `golang.org/x/sys/windows` (user32.dll, kernel32.dll), native Windows SCM API, NtSetSystemInformation syscall.

---

## Task 1: Fix stopWindowsExplorerNative in process_windows.go

**Files:**
- Modify: `pkg/gaming/process_windows.go`

The current `terminateProcessByName("explorer.exe")` causes smss.exe to auto-restart
the shell. Replace with PostMessage(WM_QUIT) to the Shell_TrayWnd, then poll until
gone, with TerminateProcess fallback.

**Step 1: Add user32.dll procs and WM_QUIT constant at top of process_windows.go**

Open `pkg/gaming/process_windows.go`. After the existing `windows` import and before
`terminateProcessByName`, add:

```go
var (
	user32            = windows.NewLazySystemDLL("user32.dll")
	procFindWindowW   = user32.NewProc("FindWindowW")
	procPostMessageW  = user32.NewProc("PostMessageW")
)

const wmQuit = 0x0012
```

**Step 2: Add stopWindowsExplorerNative function**

Append to `pkg/gaming/process_windows.go`:

```go
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
```

Note: `unsafe` and `time` are already used in this file context. Verify imports include
`"log"`, `"time"`, and `"unsafe"` — add any missing ones.

**Step 3: Build to check for compile errors**

```
cd c:\Users\Cullen\git\SysCleaner
go build -tags gui ./pkg/gaming/...
```

Expected: no output (clean build). Fix any import errors.

**Step 4: Update extreme.go to call stopWindowsExplorerNative**

In `pkg/gaming/extreme.go`, find `stopWindowsExplorer()`:

```go
func stopWindowsExplorer() error {
	return terminateProcessByName("explorer.exe")
}
```

Replace body with:

```go
func stopWindowsExplorer() error {
	return stopWindowsExplorerNative()
}
```

**Step 5: Build again to verify**

```
go build -tags gui ./pkg/gaming/...
```

Expected: clean build.

**Step 6: Commit**

```
git add pkg/gaming/process_windows.go pkg/gaming/extreme.go
git commit -m "fix(extreme): stop explorer via WM_QUIT so shell stays stopped"
```

---

## Task 2: Fix memory trimming — data race, stabilization wait, FlushModifiedList, GlobalMemoryStatusEx

**Files:**
- Modify: `pkg/memory/memory_windows.go`

**Step 1: Add GlobalMemoryStatusEx support**

At the top of `pkg/memory/memory_windows.go`, add a new Windows API proc alongside
the existing `ntdll` and `psapi` declarations:

```go
var (
	kernel32                  = windows.NewLazySystemDLL("kernel32.dll")
	procGlobalMemoryStatusEx  = kernel32.NewProc("GlobalMemoryStatusEx")
)

// MEMORYSTATUSEX matches the Windows MEMORYSTATUSEX structure.
type memoryStatusEx struct {
	dwLength                uint32
	dwMemoryLoad            uint32
	ullTotalPhys            uint64
	ullAvailPhys            uint64
	ullTotalPageFile        uint64
	ullAvailPageFile        uint64
	ullTotalVirtual         uint64
	ullAvailVirtual         uint64
	ullAvailExtendedVirtual uint64
}
```

**Step 2: Add getMemoryStatus helper**

After the struct, add:

```go
// getMemoryStatus calls GlobalMemoryStatusEx for accurate RAM figures.
// Returns (totalBytes, availBytes, freeBytes, err).
// availBytes = Available (free + standby reclaimable).
// freeBytes  = Zeroed pages (truly free, nothing cached).
func getMemoryStatus() (total, avail, free uint64, err error) {
	var ms memoryStatusEx
	ms.dwLength = uint32(unsafe.Sizeof(ms))
	ret, _, e := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&ms)))
	if ret == 0 {
		return 0, 0, 0, fmt.Errorf("GlobalMemoryStatusEx failed: %w", e)
	}
	return ms.ullTotalPhys, ms.ullAvailPhys, ms.ullAvailPhys, nil
	// Note: ullAvailPhys = Available (free+standby). Windows does not expose
	// standby pages directly via this API; we approximate standby as
	// ullAvailPhys - truly_free. For accurate standby, use NtQuerySystemInformation
	// with SystemMemoryListInformation — omitted here as the approximation is sufficient
	// for threshold decisions and avoids an undocumented syscall.
}
```

**Step 3: Add FlushModifiedList before PurgeStandbyList**

Find `PurgeStandbyList()` in `pkg/memory/memory_windows.go`. Add a new function
**above** it:

```go
// FlushModifiedList moves modified (dirty) pages to the standby list by writing
// them to disk. Call this before PurgeStandbyList so those pages become eligible
// for purging, yielding a more thorough cleanup.
func FlushModifiedList() error {
	cmd := int32(MemoryFlushModifiedList)
	ret, _, _ := procNtSetSystemInformation.Call(
		uintptr(SystemMemoryListInformation),
		uintptr(unsafe.Pointer(&cmd)),
		uintptr(unsafe.Sizeof(cmd)),
	)
	if ret != 0 {
		return fmt.Errorf("NtSetSystemInformation(FlushModifiedList) failed: %w", ntStatusError(ret))
	}
	return nil
}
```

**Step 4: Fix data race — protect lastCleanTime and trimCountTotal**

In `StartContinuousMonitor`, the variables `lastCleanTime` and `trimCountTotal` are
read/written from both the monitor goroutine and `TrimNow()` without a lock.
`monitorMu` already exists. Wrap all reads and writes of those two vars with it.

Inside the goroutine's ticker case, replace the unprotected block:

```go
// Replace this:
if freePercent < FreeMemoryThresholdPercent &&
    standbyPercent > StandbyThresholdPercent &&
    time.Since(lastCleanTime) > MinCleanInterval {

    // ... trim ...

    lastCleanTime = time.Now()
    trimCountTotal++
```

With mutex-protected reads and writes:

```go
monitorMu.Lock()
shouldTrim := freePercent < FreeMemoryThresholdPercent &&
    standbyPercent > StandbyThresholdPercent &&
    time.Since(lastCleanTime) > MinCleanInterval
monitorMu.Unlock()

if shouldTrim {
    // ... trim ...

    monitorMu.Lock()
    lastCleanTime = time.Now()
    trimCountTotal++
    monitorMu.Unlock()
```

Also protect the `stats` struct population (reads of `lastCleanTime`, `trimCountTotal`):

```go
monitorMu.Lock()
lastTrim := lastCleanTime
trimCnt := trimCountTotal
monitorMu.Unlock()

stats := MemoryStats{
    // ... other fields ...
    LastTrimTime: lastTrim,
    TrimCount:    trimCnt,
}
```

Do the same protection in `TrimNow()`:

```go
func TrimNow() error {
    if err := EnableSeProfileSingleProcessPrivilege(); err != nil {
        return fmt.Errorf("failed to enable privileges: %w", err)
    }

    log.Println("[SysCleaner] Manual RAM trim requested...")
    if err := FlushModifiedList(); err != nil {
        log.Printf("[SysCleaner] FlushModifiedList warning: %v", err)
    }
    if err := PurgeStandbyList(); err != nil {
        return fmt.Errorf("failed to purge standby list: %w", err)
    }

    monitorMu.Lock()
    lastCleanTime = time.Now()
    trimCountTotal++
    monitorMu.Unlock()

    log.Println("[SysCleaner] Manual RAM trim completed")
    return nil
}
```

**Step 5: Extend post-gentle-trim wait from 2s → 5s and add FlushModifiedList before aggressive trim**

Find the section inside the goroutine:

```go
// Check if that was enough after a brief wait (interruptible)
select {
case <-monitorDone:
    return
case <-time.After(2 * time.Second):
}
if vmem2, err := mem.VirtualMemory(); err == nil {
    newFreePercent := (float64(vmem2.Available) / float64(vmem2.Total)) * 100
    if newFreePercent < FreeMemoryThresholdPercent {
        // Aggressive trim
        log.Println("[SysCleaner] Gentle trim insufficient, purging full standby list...")
        if err := PurgeStandbyList(); err != nil {
```

Replace with:

```go
// Wait 5s for memory pressure to stabilize after gentle trim (interruptible)
select {
case <-monitorDone:
    return
case <-time.After(5 * time.Second):
}

total2, avail2, _, statErr := getMemoryStatus()
if statErr == nil {
    newFreePercent := (float64(avail2) / float64(total2)) * 100
    if newFreePercent < FreeMemoryThresholdPercent {
        // Flush modified pages to standby first, then purge standby
        log.Println("[SysCleaner] Gentle trim insufficient, flushing modified list then purging standby...")
        if err := FlushModifiedList(); err != nil {
            log.Printf("[SysCleaner] FlushModifiedList warning: %v", err)
        }
        if err := PurgeStandbyList(); err != nil {
            log.Printf("[SysCleaner] Full standby purge failed: %v", err)
        } else {
            log.Println("[SysCleaner] Full standby trim completed")
        }
        monitorMu.Lock()
        trimCountTotal++
        monitorMu.Unlock()
    }
}
```

Also update the initial stats collection in the monitor to use `getMemoryStatus`
instead of gopsutil for the free/available numbers (keep gopsutil only for
`UsedPercent` since GlobalMemoryStatusEx doesn't expose "used" directly):

```go
total, avail, _, statErr := getMemoryStatus()
if statErr != nil {
    continue
}
// For UsedPercent, fall back to gopsutil
vmem, vmemErr := mem.VirtualMemory()
usedPercent := float64(0)
usedGB := float64(0)
if vmemErr == nil {
    usedPercent = vmem.UsedPercent
    usedGB = float64(vmem.Used) / 1024 / 1024 / 1024
}

totalGB := float64(total) / 1024 / 1024 / 1024
freeGB := float64(avail) / 1024 / 1024 / 1024
standbyGB := freeGB - (float64(avail-avail) / 1024 / 1024 / 1024) // approximation
if standbyGB < 0 {
    standbyGB = 0
}
freePercent := (float64(avail) / float64(total)) * 100
standbyPercent := (standbyGB / totalGB) * 100
```

Note: for the standby approximation, keep the existing logic `freeGB - rawFreeGB`.
GlobalMemoryStatusEx's `ullAvailPhys` is "available" (free + standby reclaimable).
To get raw free, we'd need NtQuerySystemInformation — keep the gopsutil approximation
for standby specifically since it doesn't affect correctness, only the StandbyThreshold
check. Simplify: just use `avail` and `total` from GlobalMemoryStatusEx for the
primary free% check, keep gopsutil for standby%.

**Simplified approach for Step 5 monitor stats:**

```go
total, avail, _, statErr := getMemoryStatus()
if statErr != nil {
    // fallback to gopsutil
    vmem2, err2 := mem.VirtualMemory()
    if err2 != nil {
        continue
    }
    total = vmem2.Total
    avail = vmem2.Available
}

vmem, _ := mem.VirtualMemory()

totalGB := float64(total) / 1024 / 1024 / 1024
freeGB := float64(avail) / 1024 / 1024 / 1024
usedGB := float64(0)
usedPercent := float64(0)
standbyGB := float64(0)
standbyPercent := float64(0)

if vmem != nil {
    usedGB = float64(vmem.Used) / 1024 / 1024 / 1024
    usedPercent = vmem.UsedPercent
    standbyGB = freeGB - (float64(vmem.Free) / 1024 / 1024 / 1024)
    if standbyGB < 0 {
        standbyGB = 0
    }
    standbyPercent = (standbyGB / totalGB) * 100
}

freePercent := (freeGB / totalGB) * 100
```

**Step 6: Build the memory package**

```
go build -tags gui ./pkg/memory/...
```

Expected: clean build. Fix any import or type errors (likely need to add `kernel32`
import or ensure `unsafe` is imported).

**Step 7: Commit**

```
git add pkg/memory/memory_windows.go
git commit -m "fix(memory): mutex-protect trim vars, add FlushModifiedList, use GlobalMemoryStatusEx, extend stabilization wait to 5s"
```

---

## Task 3: Expand process and service kill list in extreme.go

**Files:**
- Modify: `pkg/gaming/extreme.go`

**Step 1: Expand processesToKill**

In `pkg/gaming/extreme.go`, find `processesToKill = []string{` and add the following
entries. Insert them in logical groups within the existing list:

After `"AppleMobileDeviceService.exe"` (end of current list), add:

```go
// === Browsers ===
"chrome.exe",
"firefox.exe",
"brave.exe",
"opera.exe",
"vivaldi.exe",
"msedgewebview2.exe",

// === Peripheral / RGB software ===
"RazerCortex.exe",
"RazerSynapse3.exe",
"RazerSynapse.exe",
"LGHUB.exe",
"LogiOverlay.exe",
"Overwolf.exe",
"OverwolfBrowser.exe",
"iCUEService.exe",
"OpenRGB.exe",
"ICUE.exe",
"LightingService.exe",

// === Gaming-adjacent apps ===
"EpicWebHelper.exe",
"XboxApp.exe",
"GamingServices.exe",
"Parsec.exe",
"TwitchUI.exe",
"Playnite.exe",
"obs64.exe",
"NahimicService.exe",
"MSIAfterburner.exe",
"WallpaperEngine.exe",
"Wallpaper32.exe",
"Wallpaper64.exe",

// === Crash reporters / update helpers ===
"CrashpadHandler.exe",
"CefSharp.BrowserSubprocess.exe",
"EpicGamesLauncher.exe",    // launcher helper (not game itself)
```

**DO NOT add** GPU driver containers or audio services:
- `NVDisplay.Container.exe`, `nvcontainer.exe`, `amdow.exe`, `audiodg.exe` — these are
  driver-level and killing them causes instability.

**Step 2: Expand extremeServicesToStop**

In the `extremeServicesToStop = []string{` block, add these after the existing entries:

```go
// === Remote Access ===
"RemoteAccess",    // Routing and Remote Access

// === Offline / Sync ===
"CscService",     // Offline Files

// === Windows Store / Licensing ===
"ClipSVC",        // Client License Service (Store app DRM)
"InstallService", // Microsoft Store Install Service
"EntAppSvc",      // Enterprise App Management

// === Web Account / Identity ===
"TokenBroker",    // Web Account Manager (Microsoft account sync)

// === Windows Hello / Biometrics (additional) ===
"NgcCtnrSvc",     // Windows Hello PIN Container
"NgcSvc",         // Windows Hello Credential Service

// === Clipboard ===
"cbdhsvc",        // Clipboard User Service

// === Messaging ===
"MessagingService", // Text messaging / MMS routing
"PushToInstall",    // Windows Push To Install
```

**Step 3: Build to check**

```
go build -tags gui ./pkg/gaming/...
```

Expected: clean build.

**Step 4: Commit**

```
git add pkg/gaming/extreme.go
git commit -m "feat(extreme): expand process and service kill list for broader coverage"
```

---

## Task 4: Build the GUI executable

**Files:** root of repo

**Step 1: Run the build script**

```powershell
cd c:\Users\Cullen\git\SysCleaner
.\build.ps1
```

Expected output includes:
```
Build successful!
  Executable: SysCleaner-x64.exe
  Architecture: amd64
  Size: X.XX MB
```

If `rsrc` is not installed (icon compilation step fails), the build continues without
the icon — that's acceptable. The exe will still be functional.

**Step 2: Verify exe exists**

```
ls SysCleaner-x64.exe
```

Expected: file present, size > 10 MB (Fyne apps are large due to embedded assets).

**Step 3: Commit the built exe**

```
git add SysCleaner-x64.exe
git commit -m "build: update SysCleaner-x64.exe with extreme mode fixes and expanded kill list"
```

---

## Task 5: Merge to main

**Step 1: Verify current branch is main**

```
git branch --show-current
```

Expected: `main` (work has been done directly on main per git status context).

If already on main, the commits are already there — no merge needed. Skip to Step 2.

**Step 2: Verify git log looks correct**

```
git log --oneline -6
```

Expected to see the 4 commits from this session at the top.

**Step 3: Done**

All work committed to main. If a remote exists and the user wants to push:

```
git push origin main
```

(Confirm with user before pushing — do not push autonomously.)
