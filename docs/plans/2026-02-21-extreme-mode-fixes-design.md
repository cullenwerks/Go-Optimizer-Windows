# Extreme Mode Fixes & Expansion — Design

**Date:** 2026-02-21

## Summary

Three improvements to Extreme Performance Mode:

1. Fix Windows Explorer shell not staying stopped
2. Improve memory trimming with best practices
3. Expand process/service scope for broader coverage

---

## 1. Fix: Stop Explorer Reliably

**Problem:** `terminateProcessByName("explorer.exe")` calls `TerminateProcess`, which
causes the Windows Session Manager to auto-restart explorer immediately. The shell never
actually stays stopped.

**Fix:** Replace `stopWindowsExplorer()` with a WM_QUIT approach:

1. `FindWindow("Shell_TrayWnd", nil)` — locate taskbar window handle
2. `PostMessage(hwnd, WM_QUIT, 0, 0)` — graceful quit signal
3. Poll `FindWindow` every 200ms for up to 5 seconds to confirm shell is gone
4. Fallback to `TerminateProcess` if still running after 5s

This is the standard method used by SysInternals tools. Explorer quits cleanly and
the Session Manager does not restart it because it exited with a normal quit, not a crash.

---

## 2. Fix: Trimming Best Practices

**Problems:**
- Standby estimation (`Available - Free` from gopsutil) is an approximation
- `lastCleanTime` and `trimCountTotal` are unprotected package-level vars accessed from
  concurrent goroutines (data race potential)
- Post-gentle-trim stabilization wait is 2s (too short — memory pressure takes 5s+)
- Aggressive trim does not flush modified pages first

**Fixes:**
- Protect `lastCleanTime` and `trimCountTotal` with `monitorMu` (already exists)
- Extend post-gentle-trim wait from 2s → 5s
- Add `MemoryFlushModifiedList` step before `PurgeStandbyList` in aggressive path
  (flushes modified pages to disk, making them eligible for standby purge — cleaner result)
- Use `windows.GlobalMemoryStatusEx` for accurate free RAM instead of gopsutil

---

## 3. Expand: Process and Service Scope

### Additional Processes to Kill

Browsers (memory hogs, safe to kill before gaming):
- `chrome.exe`, `firefox.exe`, `brave.exe`, `opera.exe`, `vivaldi.exe`

Gaming-adjacent launchers (not needed once game is running):
- `EpicWebHelper.exe`
- `XboxApp.exe`, `GamingServices.exe`
- `Parsec.exe`
- `TwitchUI.exe`

Peripheral software (RGB/overlay tools):
- `RazerCortex.exe`, `RazerSynapse3.exe`, `RazerSynapse.exe`
- `LGHUB.exe`, `LogiOverlay.exe`
- `Overwolf.exe`, `OverwolfBrowser.exe`
- `iCUEService.exe` (Corsair)
- `OpenRGB.exe`

Other:
- `obs64.exe` (OBS Studio)
- `Playnite.exe` (game library)
- `NahimicService.exe` (audio enhancement)
- `MSI Afterburner.exe`
- `Wallpaper32.exe`, `WallpaperEngine.exe` (wallpaper engines)

Excluded from kill list intentionally:
- GPU driver containers (`NVDisplay.Container.exe`, `nvcontainer.exe`, `amdow.exe`)
- Audio drivers (`audiodg.exe`)
- Windows Security (`SecurityHealthSystray.exe`)

### Additional Services to Stop

Windows management overhead:
- `RemoteAccess` — Routing and Remote Access
- `CscService` — Offline Files
- `DcpSvc` — Data Collection and Publishing
- `PushToInstall` — Windows PushToInstall
- `InstallService` — Microsoft Store Install Service
- `ClipSVC` — Client License Service (Store apps)
- `TokenBroker` — Web Account Manager
- `EntAppSvc` — Enterprise App Management Service

Windows Hello / biometrics:
- `NgcCtnrSvc` — Windows Hello PIN Container
- `NgcSvc` — Windows Hello Credential Service

Unnecessary user experience services:
- `ConsentUxUserSvc` — User Experience Virtualization
- `cbdhsvc` — Clipboard User Service
- `MessagingService` — Text messaging / MMS

---

## Files Changed

| File | Change |
|------|--------|
| `pkg/gaming/extreme.go` | Add processes/services; update `stopWindowsExplorer` to call new WM_QUIT function |
| `pkg/gaming/process_windows.go` | Add `stopWindowsExplorerNative()` using `user32.dll` FindWindow + PostMessage |
| `pkg/memory/memory_windows.go` | Fix data race on trim vars; extend wait; add FlushModifiedList; use GlobalMemoryStatusEx |
