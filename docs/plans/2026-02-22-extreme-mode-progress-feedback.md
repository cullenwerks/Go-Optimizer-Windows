# Extreme Mode Progress Feedback Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Show live step-by-step status text in the button during Extreme Mode activation and deactivation so users know the app hasn't frozen.

**Architecture:** Add a `func(string)` progress callback parameter to `EnableExtremeMode` and `DisableExtremeMode`. Each major step calls it with a short status string. The GUI wires this to update the toggle button text. CLI callers pass `nil`. A nil-safe helper wraps the call so no call site needs a nil check.

**Tech Stack:** Go, Fyne v2

---

### Task 1: Add progress callback to `EnableExtremeMode` and `DisableExtremeMode`

**Files:**
- Modify: `pkg/gaming/extreme.go`

**Step 1: Update `EnableExtremeMode` signature and add progress calls**

Change the function signature from:
```go
func EnableExtremeMode() error {
```
to:
```go
func EnableExtremeMode(progress func(string)) error {
```

Add a nil-safe helper at the top of the function body (before any logic):
```go
report := func(msg string) {
    if progress != nil {
        progress(msg)
    }
    log.Println("[SysCleaner]", msg)
}
```

Then replace the existing `log.Println` calls at each major step with `report(...)`:

- Before `CloseBackgroundApps`: `report("Closing background apps...")`
- After `CloseBackgroundApps` returns: `report(fmt.Sprintf("Closed %d background apps", closedCount))`
- Before the services loop: `report("Stopping non-essential services...")`
- Before `stopWindowsExplorer`: `report("Stopping Windows shell...")`
- Before `setPowerSchemeNative`: `report("Applying power plan...")`
- Before `memory.StartContinuousMonitor`: `report("Starting RAM monitor...")`

Remove the now-redundant `log.Printf("[SysCleaner] Closed %d background applications", closedCount)` and similar lines that are replaced by `report`.

**Step 2: Update `DisableExtremeMode` signature and add progress calls**

Change the signature from:
```go
func DisableExtremeMode() error {
```
to:
```go
func DisableExtremeMode(progress func(string)) error {
```

Add the same nil-safe helper at the top of the function body:
```go
report := func(msg string) {
    if progress != nil {
        progress(msg)
    }
    log.Println("[SysCleaner]", msg)
}
```

Add progress calls:
- Before `memory.StopContinuousMonitor`: `report("Stopping RAM monitor...")`
- Before `startWindowsExplorer`: `report("Restoring Windows shell...")`
- Before the services restore loop: `report("Restoring services...")`
- Before `enableVisualEffects`: `report("Restoring visual settings...")`

**Step 3: Build to confirm no errors yet (call sites will break — that's expected)**

```bash
GOOS=windows GOARCH=amd64 go build ./pkg/gaming/... 2>&1
```

Expected: PASS (the package itself compiles; callers in `cmd/` and `gui/` will break in the next step)

---

### Task 2: Update CLI call sites

**Files:**
- Modify: `cmd/extreme.go`

**Step 1: Read the file**

Open `cmd/extreme.go` and find the two call sites.

**Step 2: Update both calls to pass `nil`**

Change:
```go
gaming.EnableExtremeMode()
```
to:
```go
gaming.EnableExtremeMode(nil)
```

Change:
```go
gaming.DisableExtremeMode()
```
to:
```go
gaming.DisableExtremeMode(nil)
```

**Step 3: Build to confirm CLI compiles**

```bash
GOOS=windows GOARCH=amd64 go build ./cmd/... 2>&1
```
Expected: PASS

---

### Task 3: Update GUI call sites with live button text feedback

**Files:**
- Modify: `gui/views/extreme_mode.go`

**Step 1: Update `toggleExtremeMode` — disable path**

Locate the disable branch (around line 94). Change:
```go
if err := gaming.DisableExtremeMode(); err != nil {
```
to pass a progress callback that updates the button text:
```go
if err := gaming.DisableExtremeMode(func(msg string) {
    p.toggleBtn.SetText(msg)
}); err != nil {
```

**Step 2: Update `toggleExtremeMode` — enable path**

Locate the enable branch (around line 119). Change:
```go
if err := gaming.EnableExtremeMode(); err != nil {
```
to:
```go
if err := gaming.EnableExtremeMode(func(msg string) {
    p.toggleBtn.SetText(msg)
}); err != nil {
```

**Step 3: Build to confirm GUI compiles**

```bash
GOOS=windows GOARCH=amd64 go build -tags gui ./... 2>&1
```
Expected: PASS (no output = success)

**Step 4: Commit**

```bash
git add pkg/gaming/extreme.go cmd/extreme.go gui/views/extreme_mode.go
git commit -m "feat(extreme): add live step-by-step progress feedback during activation

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

### Task 4: Rebuild the binary

**Step 1: Run the build script**

```powershell
powershell.exe -NoProfile -Command ".\build.ps1 -Arch amd64"
```

Expected output includes:
```
Build successful!
  Executable: SysCleaner-x64.exe
```
