package gaming

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"syscleaner/pkg/admin"
	"syscleaner/pkg/memory"
)

// ExtremeMode holds state for extreme performance mode.
type ExtremeMode struct {
	ShellStopped      bool
	AntiCheatServices []string
	ClosedProcesses   []string
	ramMonitorActive  bool
}

var (
	extremeModeActive bool
	extremeMode       ExtremeMode

	// ProcessWhitelist contains process names that should never be killed.
	// Users can configure this to protect Discord, Steam, etc.
	ProcessWhitelist []string

	// Anti-cheat services that must NEVER be stopped
	antiCheatServices = []string{
		"vgc",           // Riot Vanguard
		"vgk",           // Riot Vanguard Kernel
		"EasyAntiCheat", // EasyAntiCheat
		"BEService",     // BattlEye
		"PnkBstrA",      // PunkBuster
		"PnkBstrB",      // PunkBuster
	}

	// Comprehensive list of non-essential services to stop in Extreme Gaming Mode
	// These have been verified safe to temporarily stop while gaming
	extremeServicesToStop = []string{
		// === Windows Update & Delivery ===
		"wuauserv",     // Windows Update
		"UsoSvc",       // Update Orchestrator
		"BITS",         // Background Intelligent Transfer
		"DoSvc",        // Delivery Optimization
		"WaaSMedicSvc", // Windows Update Medic

		// === Telemetry & Diagnostics ===
		"DiagTrack",        // Connected User Experiences and Telemetry
		"dmwappushservice", // WAP Push Message Routing (telemetry)
		"DPS",              // Diagnostic Policy Service
		"WdiServiceHost",   // Diagnostic Service Host
		"WdiSystemHost",    // Diagnostic System Host
		"PcaSvc",           // Program Compatibility Assistant
		"WerSvc",           // Windows Error Reporting

		// === Search & Indexing ===
		"WSearch", // Windows Search (indexer)

		// === Superfetch & Memory ===
		"SysMain", // Superfetch/SysMain (prefetch/cache manager)

		// === Sync & Cloud ===
		"OneSyncSvc", // Microsoft sync (Mail, Calendar, Contacts)

		// === Print & Fax ===
		"Spooler", // Print Spooler
		"Fax",     // Fax service

		// === Remote Access ===
		"RemoteRegistry", // Remote Registry
		"TermService",    // Remote Desktop Services
		"SessionEnv",     // Remote Desktop Configuration
		"RemoteAccess",   // Routing and Remote Access

		// === Biometrics & Security (non-essential) ===
		"WbioSrvc",  // Windows Biometric Service
		"MapsBroker", // Downloaded Maps Manager
		"lfsvc",      // Geolocation Service

		// === Phone & Mobile ===
		"PhoneSvc",  // Phone Service
		"SmsRouter", // Microsoft Windows SMS Router

		// === Touch & Tablet ===
		"TabletInputService", // Touch Keyboard and Handwriting Panel
		"WalletService",      // Wallet Service

		// === Retail & Demos ===
		"RetailDemo", // Retail Demo Service

		// === Miscellaneous ===
		"DusmSvc",              // Data Usage Service
		"wisvc",                // Windows Insider Service
		"icssvc",               // Windows Mobile Hotspot Service
		"WMPNetworkSvc",        // Windows Media Player Network Sharing
		"XblAuthManager",       // Xbox Live Auth Manager (disable if not using Xbox services)
		"XblGameSave",          // Xbox Live Game Save
		"XboxGipSvc",           // Xbox Accessory Management
		"XboxNetApiSvc",        // Xbox Live Networking Service
		"AJRouter",             // AllJoyn Router Service
		"ALG",                  // Application Layer Gateway
		"IKEEXT",               // IKE and AuthIP IPsec Keying Modules (if not using VPN)
		"iphlpsvc",             // IP Helper (IPv6 transition - safe if IPv4 only)
		"SharedAccess",         // Internet Connection Sharing
		"lmhosts",              // TCP/IP NetBIOS Helper
		"TrkWks",               // Distributed Link Tracking Client
		"WpcMonSvc",            // Parental Controls
		"SEMgrSvc",             // Payments and NFC/SE Manager
		"SCardSvr",             // Smart Card
		"ScDeviceEnum",         // Smart Card Device Enumeration
		"stisvc",               // Windows Image Acquisition (scanner/camera)
		"FrameServer",          // Windows Camera Frame Server
		"CDPSvc",               // Connected Devices Platform Service
		"CDPUserSvc",           // Connected Devices Platform User Service
		"WpnService",           // Windows Push Notifications System
		"WpnUserService",       // Windows Push Notifications User Service
		"BcastDVRUserService",  // GameDVR and Broadcast User Service (if not recording)

		// === Offline / Sync ===
		"CscService", // Offline Files

		// === Windows Store / Licensing ===
		"ClipSVC",        // Client License Service (Store app DRM)
		"InstallService", // Microsoft Store Install Service
		"EntAppSvc",      // Enterprise App Management

		// === Windows Hello / Biometrics (additional) ===
		"NgcCtnrSvc", // Windows Hello PIN Container
		"NgcSvc",     // Windows Hello Credential Service

		// === Clipboard ===
		"cbdhsvc", // Clipboard User Service

		// === Messaging ===
		"MessagingService", // Text messaging / MMS routing
		"PushToInstall",    // Windows Push To Install
	}

	// Background applications to close in Extreme Mode
	processesToKill = []string{
		"OneDrive.exe",
		"Teams.exe",
		"ms-teams.exe",
		"Spotify.exe",
		"Discord.exe",
		"DiscordPTB.exe",
		"DiscordCanary.exe",
		"Skype.exe",
		"slack.exe",
		"Cortana.exe",
		"SearchUI.exe",
		"SearchApp.exe",
		"SearchHost.exe",
		"YourPhone.exe",
		"PhoneExperienceHost.exe",
		"CalculatorApp.exe",
		"Microsoft.Photos.exe",
		"Video.UI.exe",
		"HxTsr.exe",
		"HxCalendarAppImm.exe",
		"HxOutlook.exe",
		"GameBar.exe",
		"GameBarPresenceWriter.exe",
		// NOTE: SecurityHealthSystray.exe (Windows Defender tray) is intentionally
		// NOT included — killing security software UI triggers AV heuristics
		// and is a common malware pattern.
		"PeopleApp.exe",
		"msedge.exe",
		"MicrosoftEdgeUpdate.exe",
		"GoogleCrashHandler.exe",
		"GoogleCrashHandler64.exe",
		"jusched.exe",
		"AdobeARM.exe",
		"CCleaner64.exe",
		"iCloudServices.exe",
		"AppleMobileDeviceService.exe",

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
		"Parsec.exe",
		"TwitchUI.exe",
		"Playnite.exe",
		"obs64.exe",
		"NahimicService.exe", // OEM audio DSP (MSI/ASUS) — whitelist this if you need surround sound or mic DSP
		"MSIAfterburner.exe", // NOTE: killing this stops custom fan curves; whitelist on thermally-limited systems
		"WallpaperEngine.exe",
		"Wallpaper32.exe",
		"Wallpaper64.exe",

		// === Crash reporters / update helpers ===
		"CrashpadHandler.exe",
		"CefSharp.BrowserSubprocess.exe",
		"EpicGamesLauncher.exe", // NOTE: some Epic titles (Fortnite, Rocket League) require this at runtime — whitelist if using Epic games
	}
)


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

// GetProcessesToKill returns the list of processes that would be terminated.
func GetProcessesToKill() []string {
	return processesToKill
}

// EnableExtremeMode stops Windows Explorer and non-essential services.
func EnableExtremeMode(progress func(string)) error {
	report := func(msg string) {
		if progress != nil {
			progress(msg)
		}
		log.Println("[SysCleaner]", msg)
	}

	if err := admin.RequireElevation("Extreme Performance Mode"); err != nil {
		return err
	}

	mu.Lock()
	defer mu.Unlock()

	if extremeModeActive {
		return fmt.Errorf("extreme mode already active")
	}

	if runtime.GOOS != "windows" {
		return fmt.Errorf("extreme mode only available on Windows")
	}

	// Enable regular gaming mode first if not already active
	if !gamingModeEnabled {
		mu.Unlock()
		if err := Enable(Config{AutoDetectGames: false, CPUBoost: 100, RAMReserveGB: 1}); err != nil {
			mu.Lock()
			return fmt.Errorf("failed to enable gaming mode: %w", err)
		}
		mu.Lock()
	}

	extremeMode = ExtremeMode{
		AntiCheatServices: antiCheatServices,
	}

	// Close non-essential background applications using native API
	report("Closing background apps...")
	closedCount, closedApps := CloseBackgroundApps(ProcessWhitelist)
	extremeMode.ClosedProcesses = closedApps
	report(fmt.Sprintf("Closed %d background apps", closedCount))

	// Stop additional services for extreme mode.
	report("Stopping non-essential services...")
	for i, svc := range extremeServicesToStop {
		stopService(svc)
		if i > 0 && i%3 == 0 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	// Ensure anti-cheat services are running
	report("Ensuring anti-cheat services are running...")
	for _, svc := range antiCheatServices {
		startService(svc)
	}

	// Stop Windows Explorer (Desktop Experience) via WM_CLOSE to Shell_TrayWnd.
	// This is a clean shutdown that does not trigger Session Manager auto-restart.
	report("Stopping Windows shell...")
	if err := stopWindowsExplorer(); err != nil {
		return fmt.Errorf("failed to stop explorer: %w", err)
	}
	extremeMode.ShellStopped = true

	// Set ultimate performance power plan
	report("Applying power plan...")
	if err := setPowerSchemeNative("8c5e7fda-e8bf-4a96-9a85-a6e23a8c635c"); err != nil {
		log.Printf("[SysCleaner] Failed to set ultimate performance power plan: %v", err)
	}

	// Disable visual effects for maximum performance using native registry API
	disableVisualEffects()

	// Start RAM monitoring for automatic standby trimming
	report("Starting RAM monitor...")
	if err := memory.EnableSeProfileSingleProcessPrivilege(); err != nil {
		log.Printf("[SysCleaner] Warning: Failed to enable memory privileges: %v", err)
		log.Println("[SysCleaner] RAM trimming may not work correctly. Run as Administrator.")
	}
	memory.StartContinuousMonitor(nil)
	extremeMode.ramMonitorActive = true

	extremeModeActive = true
	writeSentinel()
	log.Println("[SysCleaner] Extreme Mode ACTIVATED - Maximum performance enabled")
	return nil
}

// DisableExtremeMode restores Windows Explorer and services.
func DisableExtremeMode(progress func(string)) error {
	report := func(msg string) {
		if progress != nil {
			progress(msg)
		}
		log.Println("[SysCleaner]", msg)
	}

	mu.Lock()
	defer mu.Unlock()

	if !extremeModeActive {
		return fmt.Errorf("extreme mode not active")
	}

	// Stop RAM monitoring
	if extremeMode.ramMonitorActive {
		report("Stopping RAM monitor...")
		memory.StopContinuousMonitor()
		extremeMode.ramMonitorActive = false
	}

	// Restart Windows Explorer first
	if extremeMode.ShellStopped {
		report("Restoring Windows shell...")
		if err := startWindowsExplorer(); err != nil {
			return fmt.Errorf("failed to restart explorer: %w", err)
		}
	}

	// Restore services with pacing
	report("Restoring services...")
	for i, svc := range extremeServicesToStop {
		startService(svc)
		if i > 0 && i%3 == 0 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	// Re-enable visual effects
	report("Restoring visual settings...")
	enableVisualEffects()

	extremeModeActive = false
	deleteSentinel()

	// Disable regular gaming mode
	mu.Unlock()
	err := Disable()
	mu.Lock()

	log.Println("[SysCleaner] Extreme Mode DEACTIVATED - Normal operation restored")
	return err
}

// IsExtremeModeActive returns extreme mode status.
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

// CloseBackgroundApps closes non-essential background applications.
// Returns the count of closed apps and a list of closed process names.
// Uses native TerminateProcess API instead of taskkill.exe to avoid
// triggering AV heuristics from rapid child process spawning.
// Processes in the whitelist are skipped.
func CloseBackgroundApps(whitelist []string) (int, []string) {
	closed := 0
	closedApps := []string{}

	whitelistMap := make(map[string]bool)
	for _, name := range whitelist {
		whitelistMap[strings.ToLower(name)] = true
	}

	for i, processName := range processesToKill {
		if whitelistMap[strings.ToLower(processName)] {
			log.Printf("[SysCleaner] Skipping whitelisted process: %s", processName)
			continue
		}

		if err := terminateProcessByName(processName); err == nil {
			closed++
			closedApps = append(closedApps, processName)
			log.Printf("[SysCleaner] Closed: %s", processName)
		}
		// Batch delay: 500ms every 3 processes to avoid AV heuristics
		if i > 0 && i%3 == 0 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	return closed, closedApps
}

func stopWindowsExplorer() error {
	return stopWindowsExplorerNative()
}

func startWindowsExplorer() error {
	return startExplorerNative()
}

// disableVisualEffects uses native registry API on Windows, falls back to reg.exe
func disableVisualEffects() {
	setVisualEffectsNative(false)
}

// enableVisualEffects uses native registry API on Windows, falls back to reg.exe
func enableVisualEffects() {
	setVisualEffectsNative(true)
}

