# Linting, UX Polish & Icon Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix code quality issues, polish key UX rough edges, create a flame+gear SVG icon with CI-based ICO generation, and add a branded banner to the README.

**Architecture:** Three independent tracks (A: code quality, B: UX polish, C: icon/branding) executed sequentially as separate tasks. No new dependencies required — icon conversion uses ImageMagick which is pre-installed on GitHub runners and `magick` locally. UX changes are all within existing Fyne widget primitives.

**Tech Stack:** Go, Fyne v2, SVG (hand-authored XML), ImageMagick (`magick`), GitHub Actions, PowerShell.

---

## Task 1: Power scheme GUID constants (Track A)

**Files:**
- Modify: `pkg/gaming/gaming.go`
- Modify: `pkg/gaming/extreme.go`

**Step 1: Add constants to `pkg/gaming/gaming.go`**

Add these two constants immediately before the `var` block (after the `import` block):

```go
const (
	// powerSchemeHighPerformance is the Windows "High Performance" power plan GUID.
	powerSchemeHighPerformance = "8c5e7fda-e8bf-4a96-9a85-a6e23a8c635c"
	// powerSchemeBalanced is the Windows "Balanced" power plan GUID.
	powerSchemeBalanced = "381b4222-f694-41f0-9685-ff5bb260df2e"
)
```

**Step 2: Replace bare strings in `gaming.go`**

In `Enable()` (around line 96), replace:
```go
if err := setPowerSchemeNative("8c5e7fda-e8bf-4a96-9a85-a6e23a8c635c"); err != nil {
```
with:
```go
if err := setPowerSchemeNative(powerSchemeHighPerformance); err != nil {
```

In `Disable()` (around line 144), replace:
```go
if err := setPowerSchemeNative("381b4222-f694-41f0-9685-ff5bb260df2e"); err != nil {
```
with:
```go
if err := setPowerSchemeNative(powerSchemeBalanced); err != nil {
```

**Step 3: Replace bare strings in `extreme.go`**

In `EnableExtremeMode()` (the `setPowerSchemeNative` call), replace the bare GUID string with `powerSchemeHighPerformance`.

Note: `extreme.go` is in package `gaming` so the constant is directly accessible — no import needed.

**Step 4: Build**
```bash
cd c:\Users\Cullen\git\SysCleaner && go build -tags gui ./...
```
Expected: no errors.

**Step 5: Commit**
```bash
git add pkg/gaming/gaming.go pkg/gaming/extreme.go
git commit -m "refactor: replace magic power scheme GUID strings with named constants"
```

---

## Task 2: CREATE_NO_WINDOW and flag comment cleanup (Track A)

**Files:**
- Modify: `pkg/gaming/worker_windows.go`
- Modify: `cmd/clean.go`
- Modify: `cmd/optimize.go`
- Modify: `cmd/gaming.go`
- Modify: `cmd/extreme.go`
- Modify: `pkg/priority/priority.go`

**Step 1: Replace `0x08000000` in `worker_windows.go`**

Find the line:
```go
const createNoWindow = 0x08000000
```
Delete it. Then in the `windows.CreateProcess` call, replace `createNoWindow` with `windows.CREATE_NO_WINDOW`.

**Step 2: Add flag comment to each cmd file**

In each of `cmd/clean.go`, `cmd/optimize.go`, `cmd/gaming.go`, add a comment directly above the block of `GetBool(_, _)` calls:

```go
// Flag lookup errors are impossible here: all flags are registered in init().
```

`cmd/extreme.go` already has the flag block clearly structured — add the same comment there too.

**Step 3: Add priority value 4 comment to `pkg/priority/priority.go`**

Find the `GetCpuPriorityName` function. Above the `switch value` line, or in the comment block, add:

```go
// Windows CPU priority class values are: 1 (Idle), 2 (Normal), 3 (High),
// 5 (Below Normal), 6 (Above Normal). Value 4 is not defined by the Windows API
// (the enum jumps from 3 to 5); it is deliberately omitted here.
```

**Step 4: Build**
```bash
go build -tags gui ./...
```
Expected: no errors.

**Step 5: Commit**
```bash
git add pkg/gaming/worker_windows.go cmd/clean.go cmd/optimize.go cmd/gaming.go cmd/extreme.go pkg/priority/priority.go
git commit -m "refactor: use windows.CREATE_NO_WINDOW constant; document flag discard and priority gap"
```

---

## Task 3: Priority panel — disable button during op + fix header (Track B)

**Files:**
- Modify: `gui/views/priority_panel.go`

**Step 1: Read the file**

Read `gui/views/priority_panel.go` to confirm current line numbers.

**Step 2: Disable Apply button during operation**

The `applyBtn` is currently declared and its `OnTapped` set inline. The issue is `applyBtn` is declared as a value (`applyBtn := widget.NewButton(...)`) with the full handler inline, so self-reference is possible via closure.

Replace the `applyBtn` declaration block (the `widget.NewButton("Apply Priority", func() { ... })` block) with:

```go
applyBtn := widget.NewButton("Apply Priority", nil)
applyBtn.OnTapped = func() {
    processName := strings.TrimSpace(processNameEntry.Text)
    if processName == "" {
        dialog.ShowError(fmt.Errorf("process name cannot be empty"), w)
        return
    }

    applyBtn.Disable()
    go func() {
        defer applyBtn.Enable()

        cpuPriorityName := cpuSelect.Selected
        ioPriorityName := ioSelect.Selected
        pagePriorityName := pageSelect.Selected

        cpuVal := priority.ParseCpuPriorityName(cpuPriorityName)
        ioVal := priority.ParseIoPriorityName(ioPriorityName)
        pageVal := priority.ParsePagePriorityName(pagePriorityName)

        if err := priority.SetProcessPriority(processName, cpuVal, ioVal, pageVal); err != nil {
            dialog.ShowError(err, w)
            return
        }

        dialog.ShowInformation("Success",
            fmt.Sprintf("Priority settings applied to %s\nChanges take effect next time the process starts", processName), w)
        refreshTable()
        processNameEntry.SetText("")
    }()
}
```

**Step 3: Replace monospace header with proper grid header**

Find the `header` label declaration:
```go
header := widget.NewLabel("Process Name            CPU Priority       I/O Priority       Page Priority")
header.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
```

Replace with a proper 4-column grid that matches the table column widths (250, 150, 150, 150):
```go
header := container.NewGridWithColumns(4,
    widget.NewLabelWithStyle("Process Name", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
    widget.NewLabelWithStyle("CPU Priority", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
    widget.NewLabelWithStyle("I/O Priority", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
    widget.NewLabelWithStyle("Page Priority", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
)
```

Note: `header` is now a `fyne.CanvasObject` (container), not a `*widget.Label`. Find where it's used in the layout and confirm it's just added to a `container.NewVBox` or `container.NewBorder` — it should work as-is since both are `fyne.CanvasObject`.

**Step 4: Build**
```bash
go build -tags gui ./...
```
Expected: no errors.

**Step 5: Commit**
```bash
git add gui/views/priority_panel.go
git commit -m "fix: disable Apply button during async priority op; replace monospace header with grid"
```

---

## Task 4: Dashboard error states (Track B)

**Files:**
- Modify: `gui/views/dashboard.go`

**Step 1: Fix system info goroutine**

Find the `host.Info()` goroutine (around line 47):
```go
go func() {
    if info, err := host.Info(); err == nil {
        sysInfoLabel.SetText(...)
    }
}()
```

Replace with:
```go
go func() {
    info, err := host.Info()
    if err != nil {
        sysInfoLabel.SetText("System info unavailable")
        return
    }
    sysInfoLabel.SetText(fmt.Sprintf("OS: %s %s | Hostname: %s | Uptime: %s",
        info.Platform, info.PlatformVersion, info.Hostname,
        (time.Duration(info.Uptime)*time.Second).String()))
}()
```

**Step 2: Fix CPU metric error state**

In the real-time ticker goroutine, find:
```go
if cpuPercent, err := cpu.Percent(0, false); err == nil && len(cpuPercent) > 0 {
    ...
    cpuLabel.SetText(fmt.Sprintf("CPU: %.1f%%", smoothCPU*100))
}
```

Replace with:
```go
if cpuPercent, err := cpu.Percent(0, false); err == nil && len(cpuPercent) > 0 {
    targetCPU := cpuPercent[0] / 100.0
    smoothCPU := prevCPU + (targetCPU-prevCPU)*0.3
    prevCPU = smoothCPU
    cpuBar.SetValue(smoothCPU)
    cpuLabel.SetText(fmt.Sprintf("CPU: %.1f%%", smoothCPU*100))
} else {
    cpuLabel.SetText("CPU: – (unavailable)")
}
```

**Step 3: Fix RAM metric error state**

Same pattern for the `mem.VirtualMemory()` block:
```go
if vmem, err := mem.VirtualMemory(); err == nil {
    ...
} else {
    ramLabel.SetText("RAM: – (unavailable)")
}
```

**Step 4: Fix Disk metric error state**

Same pattern for `disk.Usage("/")`:
```go
if usage, err := disk.Usage("/"); err == nil {
    ...
} else {
    diskLabel.SetText("Disk: – (unavailable)")
}
```

Note: On Windows the root path is `"/"` but gopsutil handles this correctly. No change needed to the path.

**Step 5: Build**
```bash
go build -tags gui ./...
```
Expected: no errors.

**Step 6: Commit**
```bash
git add gui/views/dashboard.go
git commit -m "fix: show error state labels when dashboard metrics fail to load"
```

---

## Task 5: Clean panel — accordion sections (Track B)

**Files:**
- Modify: `gui/views/clean_panel.go`

The current clean panel has three flat sections (System, Browsers, Apps) each with a header+grid, all always visible. We wrap each section's grid in a `widget.AccordionItem` so it collapses. The "Select All / Deselect All" buttons move inside the accordion content.

**Step 1: Read the file**

Read `gui/views/clean_panel.go` to get current line numbers before editing.

**Step 2: Replace the three section builds + final `content` VBox**

The current code builds `systemHeader`, `systemGrid`, `browserHeader`, `browserGrid`, `appHeader`, `appGrid` and then assembles them in `content`. Replace all of that (from the `// System section with select all/deselect all` comment to the `return` statement) with:

```go
	// System accordion section
	systemContent := container.NewVBox(
		container.NewHBox(sysSelectAll, sysDeselectAll),
		container.NewGridWithColumns(4,
			winTempCheck, userTempCheck, prefetchCheck, crashDumpCheck,
			errorReportsCheck, thumbCacheCheck, iconCacheCheck, shaderCacheCheck,
			dnsCacheCheck, winLogsCheck, eventLogsCheck, deliveryOptCheck,
			recycleBinCheck, winUpdateCheck, winInstallerCheck, fontCacheCheck,
		),
	)

	// Browser accordion section
	browserContent := container.NewVBox(
		container.NewHBox(browserSelectAll, browserDeselectAll),
		container.NewGridWithColumns(5,
			chromeCheck, firefoxCheck, edgeCheck, braveCheck, operaCheck,
		),
	)

	// Apps accordion section
	appContent := container.NewVBox(
		container.NewHBox(appSelectAll, appDeselectAll),
		container.NewGridWithColumns(3,
			discordCheck, spotifyCheck, steamCheck,
			teamsCheck, vscodeCheck, javaCheck,
		),
	)

	accordion := widget.NewAccordion(
		widget.NewAccordionItem("System (16 items)", systemContent),
		widget.NewAccordionItem("Browsers (5 items)", browserContent),
		widget.NewAccordionItem("Applications (6 items)", appContent),
	)
	// All sections collapsed by default — user opens what they need.

	content := container.NewVBox(
		widget.NewLabelWithStyle("System Cleaning", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		accordion,
		widget.NewSeparator(),
		buttonRow,
		widget.NewSeparator(),
		statusLabel,
		progressBar,
		resultText,
	)

	return container.NewScroll(container.NewPadded(content))
```

The `sysSelectAll`, `sysDeselectAll`, `browserSelectAll`, `browserDeselectAll`, `appSelectAll`, `appDeselectAll` buttons are still created earlier in the function — keep those. Just move them inside the accordion content.

**Step 3: Build**
```bash
go build -tags gui ./...
```
Expected: no errors.

**Step 4: Commit**
```bash
git add gui/views/clean_panel.go
git commit -m "feat: collapse clean panel categories into accordion sections"
```

---

## Task 6: Extreme Mode dialog trim + Optimize per-step feedback (Track B)

**Files:**
- Modify: `gui/views/extreme_mode.go`
- Modify: `gui/views/optimize_panel.go`

**Step 1: Trim Extreme Mode confirmation dialog**

In `gui/views/extreme_mode.go`, find the `dialog.ShowConfirm` call in `toggleExtremeMode`. The current body string is:
```go
"This will:\n\n"+
    "  - Stop Windows Explorer (no desktop/taskbar)\n"+
    "  - Stop all non-essential services\n"+
    "  - Close background apps (respecting whitelist)\n"+
    "  - Maximize game performance\n\n"+
    "You can only launch games from this window.\nContinue?",
```

Replace with:
```go
"This will:\n\n"+
    "  - Stop Windows Explorer (no desktop/taskbar)\n"+
    "  - Stop non-essential services\n"+
    "  - Close background apps (respecting whitelist)\n\n"+
    "Continue?",
```

**Step 2: Add per-step feedback to Optimize panel**

In `gui/views/optimize_panel.go`, replace the `allBtn.OnTapped` goroutine body with per-step status updates:

```go
allBtn.OnTapped = func() {
    disableAll()
    progressBar.Show()
    progressBar.Start()
    statusLabel.SetText("Running all optimizations...")

    go func() {
        defer enableAll()
        text := ""

        statusLabel.SetText("Optimizing startup programs...")
        startupResult := optimizer.OptimizeStartup()
        text += fmt.Sprintf("Startup: %d programs disabled\n", startupResult.Disabled)

        statusLabel.SetText("Optimizing network settings...")
        netResult := optimizer.OptimizeNetwork()
        text += fmt.Sprintf("Network: %dms latency reduction, %d optimizations\n",
            netResult.LatencyReduction, len(netResult.Optimizations))

        statusLabel.SetText("Optimizing disk settings...")
        diskResult := optimizer.OptimizeDisk()
        diskType := "HDD"
        if diskResult.IsSSD {
            diskType = "SSD"
        }
        text += fmt.Sprintf("Disk: %s optimized\n", diskType)

        progressBar.Stop()
        progressBar.Hide()
        statusLabel.SetText("All optimizations complete!")
        resultText.SetText(text)
    }()
}
```

**Step 3: Build**
```bash
go build -tags gui ./...
```
Expected: no errors.

**Step 4: Commit**
```bash
git add gui/views/extreme_mode.go gui/views/optimize_panel.go
git commit -m "fix: trim extreme mode dialog; add per-step status to optimize panel"
```

---

## Task 7: Create the flame+gear SVG icon (Track C)

**Files:**
- Create: `assets/icon.svg`

**Step 1: Create `assets/icon.svg`**

Write this exact SVG to `assets/icon.svg`:

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 256 256" width="256" height="256">
  <defs>
    <linearGradient id="flameGrad" x1="0.5" y1="1" x2="0.5" y2="0">
      <stop offset="0%" stop-color="#FF5500"/>
      <stop offset="100%" stop-color="#DC1E1E"/>
    </linearGradient>
    <linearGradient id="gearGrad" x1="0" y1="0" x2="1" y2="1">
      <stop offset="0%" stop-color="#2A2A2A"/>
      <stop offset="100%" stop-color="#1A1A1A"/>
    </linearGradient>
  </defs>

  <!-- 8-tooth gear centered at 128,148 -->
  <!-- Outer gear shape using polygon approximation for 8 teeth -->
  <g transform="translate(128,148)">
    <!-- Gear body -->
    <path d="
      M0,-95 L12,-88 L18,-72 L32,-72 L40,-85 L52,-78 L50,-63 L62,-53 L76,-58
      L82,-46 L72,-35 L75,-20 L90,-14 L90,0 L75,6 L72,21 L82,32 L76,44
      L62,39 L50,49 L52,64 L40,71 L32,58 L18,58 L12,72 L0,79
      L-12,72 L-18,58 L-32,58 L-40,71 L-52,64 L-50,49 L-62,39
      L-76,44 L-82,32 L-72,21 L-75,6 L-90,0 L-90,-14 L-75,-20
      L-72,-35 L-82,-46 L-76,-58 L-62,-53 L-50,-63 L-52,-78
      L-40,-85 L-32,-72 L-18,-72 L-12,-88 Z
    " fill="url(#gearGrad)"/>
    <!-- Inner bore hole -->
    <circle cx="0" cy="0" r="32" fill="#111111"/>
    <!-- Inner ring for depth -->
    <circle cx="0" cy="0" r="36" fill="none" stroke="#333333" stroke-width="3"/>
    <!-- Circuit accent: thin line on top tooth -->
    <line x1="-8" y1="-88" x2="8" y2="-88" stroke="#FF5500" stroke-width="2"/>
    <rect x="-9" y="-91" width="4" height="4" fill="#FF5500"/>
    <rect x="5" y="-91" width="4" height="4" fill="#FF5500"/>
  </g>

  <!-- Flame rising from gear center, base at y=148, tip near y=40 -->
  <path d="
    M128,148
    C118,138 108,118 114,95
    C116,85 110,75 104,68
    C108,80 106,90 112,98
    C100,80 96,58 104,40
    C108,55 116,65 120,58
    C122,48 128,38 128,38
    C128,38 134,48 136,58
    C140,65 148,55 152,40
    C160,58 156,80 144,98
    C150,90 148,80 152,68
    C146,75 140,85 142,95
    C148,118 138,138 128,148 Z
  " fill="url(#flameGrad)" opacity="0.95"/>
</svg>
```

**Step 2: Verify the file was created**
```bash
ls assets/icon.svg
```
Expected: file exists, ~2KB.

**Step 3: Commit**
```bash
git add assets/icon.svg
git commit -m "feat: add flame+gear SVG icon for SysCleaner"
```

---

## Task 8: CI — SVG to ICO conversion (Track C)

**Files:**
- Modify: `.github/workflows/release.yml`
- Modify: `.github/workflows/ci.yml`
- Modify: `build.ps1`

**Step 1: Add SVG→ICO step to `release.yml`**

Find the `Embed icon resource` step in `release.yml`:
```yaml
      - name: Embed icon resource
        if: hashFiles('assets/icon.ico') != ''
        shell: bash
        run: |
          rsrc -ico assets/icon.ico -arch ${{ matrix.arch }} -o rsrc_windows_${{ matrix.arch }}.syso
```

Insert a new step immediately BEFORE it:
```yaml
      - name: Generate icon ICO from SVG
        shell: bash
        run: |
          magick assets/icon.svg \
            -define icon:auto-resize=256,48,32,16 \
            assets/icon.ico

      - name: Embed icon resource
        if: hashFiles('assets/icon.ico') != ''
        shell: bash
        run: |
          rsrc -ico assets/icon.ico -arch ${{ matrix.arch }} -o rsrc_windows_${{ matrix.arch }}.syso
```

**Step 2: Add SVG→ICO step to `ci.yml`**

Find the `Build check (GUI)` step in `ci.yml`. Insert a new step before it:
```yaml
      - name: Generate icon ICO from SVG
        shell: bash
        run: |
          magick assets/icon.svg \
            -define icon:auto-resize=256,48,32,16 \
            assets/icon.ico

      - name: Build check (GUI)
        ...
```

**Step 3: Add optional SVG→ICO step to `build.ps1`**

In `build.ps1`, find the `Compile icon resource for this architecture` block inside `Build-SysCleaner` function:
```powershell
    if ($WithIcon -and (Test-Path "assets/icon.ico")) {
```

Insert before this block:
```powershell
    # Auto-generate icon.ico from icon.svg if ImageMagick is available
    if ((Test-Path "assets/icon.svg") -and -not (Test-Path "assets/icon.ico")) {
        if (Get-Command magick -ErrorAction SilentlyContinue) {
            Write-Host "  Converting icon.svg to icon.ico via ImageMagick..." -ForegroundColor Yellow
            magick assets/icon.svg -define icon:auto-resize=256,48,32,16 assets/icon.ico
        }
    }
```

**Step 4: Build to confirm workflow YAML is valid**

GitHub Actions YAML is validated on push. Locally, just confirm the files look right:
```bash
go build -tags gui ./...
```
Expected: no errors (this doesn't validate YAML but catches any Go issues).

**Step 5: Commit**
```bash
git add .github/workflows/release.yml .github/workflows/ci.yml build.ps1
git commit -m "feat: auto-generate icon.ico from icon.svg in CI and local builds"
```

---

## Task 9: README banner and icon display (Track C)

**Files:**
- Modify: `README.md`

**Step 1: Read the README**

Read `README.md` to see the current top of the file.

**Step 2: Add icon/banner to README header**

The README currently starts with:
```markdown
# SysCleaner v2.0

> **Free, open-source Windows system optimizer with extreme gaming mode**
```

Replace the title block with a centered banner that includes the SVG icon and project name:

```markdown
<p align="center">
  <img src="assets/icon.svg" alt="SysCleaner" width="96" height="96"/>
</p>

<h1 align="center">SysCleaner</h1>

<p align="center">
  <strong>Free, open-source Windows system optimizer with extreme gaming mode</strong>
</p>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License"/></a>
  <img src="https://img.shields.io/badge/go-1.21+-blue.svg" alt="Go Version"/>
  <img src="https://img.shields.io/badge/platform-Windows%2010%2F11-blue.svg" alt="Platform"/>
  <img src="https://img.shields.io/badge/arch-x64%20%7C%20ARM64-blue.svg" alt="Arch"/>
</p>

---

**SysCleaner** is a comprehensive Windows optimization tool built for gamers and power users. With an intuitive GUI, automated RAM monitoring, CPU priority management, and extreme performance modes, it delivers everything you need to maximize your system's potential.
```

Remove the old badge lines (lines 6–9 with the `[![...](...)]` syntax) since they're now in the centered block.

**Step 3: Commit**
```bash
git add README.md
git commit -m "docs: add icon banner to README header"
```

---

## Task 10: Run tests and final build (verification)

**Step 1: Run tests**
```bash
cd c:\Users\Cullen\git\SysCleaner && go test ./...
```
Expected: all pass.

**Step 2: Build release binary**
```powershell
./build.ps1
```
Expected: `SysCleaner-x64.exe` built successfully. If ImageMagick is installed locally, `assets/icon.ico` will be generated and embedded automatically. If not, the build proceeds without icon (same as before).

**Step 3: Verify git log**
```bash
git log --oneline -10
```
Expected: 9 commits from this work visible.

**Step 4: Push**
```bash
git push
```

---

## Notes for Implementer

- `widget.Accordion` is part of Fyne v2 core — no new import needed.
- The SVG uses a hand-authored gear path (not a perfect mathematical gear). The path approximation is intentional for simplicity; the result is visually a gear. If the SVG renders badly, the flame is the key visual element and the gear is secondary.
- `windows.CREATE_NO_WINDOW` is defined in `golang.org/x/sys/windows` as `0x08000000` — same value, just named.
- The `header` in `priority_panel.go` changes type from `*widget.Label` to `fyne.CanvasObject` (a container). It is only used as a child of another container, so this type change is transparent.
- On GitHub runners, `magick` (ImageMagick v7) is pre-installed on `windows-latest`. The `magick` command is the unified v7 CLI (replacing the old `convert` command).
- The README SVG reference (`assets/icon.svg`) renders inline on GitHub because GitHub's Markdown renderer supports SVG in `<img>` tags from the same repo.
