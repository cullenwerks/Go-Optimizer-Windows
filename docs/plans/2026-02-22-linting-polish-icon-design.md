# Linting, UX Polish & Icon Design

**Date:** 2026-02-22

**Goal:** Fix code quality issues surfaced by audit, polish UX quick wins and layout, and add a flame+gear application icon generated from SVG in CI.

---

## Track A — Code Quality / Linting

### A1. Power scheme GUID constants
Add to `pkg/gaming/gaming.go` as package-level constants, replace all 4 bare string usages across `gaming.go` and `extreme.go`:
```go
const (
    powerSchemeHighPerformance = "8c5e7fda-e8bf-4a96-9a85-a6e23a8c635c"
    powerSchemeBalanced        = "381b4222-f694-41f0-9685-ff5bb260df2e"
)
```

### A2. CREATE_NO_WINDOW constant
Replace `0x08000000` literal in `pkg/gaming/worker_windows.go` with `windows.CREATE_NO_WINDOW` (exists in `golang.org/x/sys/windows`).

### A3. cmd flag error comments
The `GetBool(_, _)` discards in `cmd/clean.go`, `cmd/optimize.go`, `cmd/gaming.go`, `cmd/extreme.go` are safe because flags are always registered in `init()` — errors are structurally impossible. Add a one-line comment above each block making this explicit rather than appearing sloppy.

### A4. Priority value 4 documentation
Add a comment in `pkg/priority/priority.go` explaining that Windows priority class values jump from 3 to 5 (value 4 does not correspond to any Windows `PRIORITY_CLASS` constant).

### A5. Priority panel Apply button disable
In `gui/views/priority_panel.go`, add `applyBtn.Disable()` at the start of the Apply handler and `defer applyBtn.Enable()` so the button cannot be double-clicked during an async operation.

---

## Track B — UX Polish

### B1. Dashboard error states
Wrap `host.Info()` and the metrics goroutines so failures set visible labels:
- System info: "System info unavailable" (instead of frozen "Loading system info...")
- CPU/RAM/Disk: show "–" with a note "(unavailable)" instead of silent blank on gopsutil failure

### B2. Priority panel header
Replace the monospace-padded header string with a proper `container.NewGridWithColumns(4, ...)` row of bold labels — same 4 columns (Process Name, CPU Priority, I/O Priority, Page Priority), no manual spacing.

### B3. Clean panel — collapsible accordion sections
Replace the flat 27-checkbox grid with three `widget.Accordion` entries:
- **System** (16 items) — collapsed by default
- **Browsers** (5 items) — collapsed by default
- **Apps** (6 items) — collapsed by default

Each section has a "Select All / None" toggle button above its checkboxes. This eliminates the wall-of-checkboxes overwhelm without removing functionality.

### B4. Extreme Mode confirmation dialog
Trim the confirmation message from 5 verbose bullet lines to 3 concise ones:
```
- Stops Windows Explorer (no desktop/taskbar)
- Stops non-essential services
- Closes background apps (respecting whitelist)
```
Remove the "You can only launch games from this window. Continue?" sentence — it's already implied by the context.

### B5. Optimize panel per-step feedback
Replace the single generic "Optimizing..." progress message with per-step status labels that update as each operation completes:
- "Optimizing startup programs..." → "✓ Startup programs"
- "Optimizing network settings..." → "✓ Network settings"
- "Optimizing disk settings..." → "✓ Disk settings"

Use a `widget.Label` per step that changes text and style on completion.

---

## Track C — Icon

### C1. SVG icon file
Create `assets/icon.svg` — a 256×256 flame+gear design:
- **Gear:** Dark charcoal (#1A1A1A) 8-tooth gear, centered, ~200px outer diameter, ~120px inner diameter. A lighter charcoal inner ring (#2A2A2A) gives subtle depth.
- **Flame:** 3-lobe stylized flame rising from gear center, SVG `linearGradient` from orange (#FF5500) at base to red (#DC1E1E) at tip. Flame extends ~90px above gear center.
- **Circuit accent:** A thin (#FF5500, 1.5px) horizontal line along the top gear tooth, with two small square pads — a minimal circuit-trace nod.
- **Background:** Transparent (no `<rect>` background fill).

### C2. CI — SVG → ICO conversion
Add an ImageMagick `convert` step to both `release.yml` and `ci.yml` (before the `rsrc` embed step) that produces `assets/icon.ico` from `assets/icon.svg` with all 4 required sizes:
```bash
magick assets/icon.svg \
  -define icon:auto-resize=256,48,32,16 \
  assets/icon.ico
```
ImageMagick (`magick`) is pre-installed on all GitHub `windows-latest` runners.

### C3. build.ps1 — local icon generation
Add an optional pre-build step to `build.ps1` that checks for `magick` (ImageMagick) and if present, converts `assets/icon.svg` → `assets/icon.ico` before the `rsrc` step. If `magick` is not installed locally, skip silently (existing behaviour — build without icon).

---

## Files Changed

| File | Change |
|------|--------|
| `pkg/gaming/gaming.go` | Add power scheme constants, use them |
| `pkg/gaming/extreme.go` | Use power scheme constants |
| `pkg/gaming/worker_windows.go` | Use `windows.CREATE_NO_WINDOW` |
| `cmd/clean.go`, `cmd/optimize.go`, `cmd/gaming.go`, `cmd/extreme.go` | Add clarifying comments on flag error discards |
| `pkg/priority/priority.go` | Add comment explaining missing value 4 |
| `gui/views/priority_panel.go` | Disable apply button during op; fix header |
| `gui/views/dashboard.go` | Error states for metrics and system info |
| `gui/views/clean_panel.go` | Accordion sections for checkboxes |
| `gui/views/extreme_mode.go` | Trim confirmation dialog text |
| `gui/views/optimize_panel.go` | Per-step status labels |
| `assets/icon.svg` | New file — flame+gear icon |
| `.github/workflows/release.yml` | Add SVG→ICO conversion step |
| `.github/workflows/ci.yml` | Add SVG→ICO conversion step |
| `build.ps1` | Optional local SVG→ICO step |
