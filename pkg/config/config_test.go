package config

import (
	"os"
	"testing"

	"syscleaner/pkg/cleaner"
)

func TestDefaultConfig_ReturnsReasonableDefaults(t *testing.T) {
	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	// ProcessWhitelist should be non-nil but empty.
	if cfg.ProcessWhitelist == nil {
		t.Error("ProcessWhitelist should not be nil")
	}
	if len(cfg.ProcessWhitelist) != 0 {
		t.Errorf("expected empty ProcessWhitelist, got %v", cfg.ProcessWhitelist)
	}

	// Some sensible cleaning options should be enabled by default.
	opts := cfg.DefaultCleanOptions
	if !opts.WindowsTemp {
		t.Error("expected WindowsTemp to be true by default")
	}
	if !opts.UserTemp {
		t.Error("expected UserTemp to be true by default")
	}
	if !opts.Prefetch {
		t.Error("expected Prefetch to be true by default")
	}
	if !opts.ThumbnailCache {
		t.Error("expected ThumbnailCache to be true by default")
	}
	if !opts.DNSCache {
		t.Error("expected DNSCache to be true by default")
	}
	if !opts.ChromeCache {
		t.Error("expected ChromeCache to be true by default")
	}
	if !opts.FirefoxCache {
		t.Error("expected FirefoxCache to be true by default")
	}
	if !opts.EdgeCache {
		t.Error("expected EdgeCache to be true by default")
	}

	// Potentially destructive or heavyweight options should be off by default.
	if opts.RecycleBin {
		t.Error("expected RecycleBin to be false by default")
	}
	if opts.EventLogs {
		t.Error("expected EventLogs to be false by default")
	}
	if opts.DryRun {
		t.Error("expected DryRun to be false by default")
	}

	// RAM monitor thresholds should be positive.
	if cfg.RAMMonitor.FreeThresholdPercent <= 0 {
		t.Errorf("expected positive FreeThresholdPercent, got %f", cfg.RAMMonitor.FreeThresholdPercent)
	}
	if cfg.RAMMonitor.StandbyThresholdPercent <= 0 {
		t.Errorf("expected positive StandbyThresholdPercent, got %f", cfg.RAMMonitor.StandbyThresholdPercent)
	}

	// UI preferences should have a sensible default tab.
	if cfg.UIPreferences.LastActiveTab == "" {
		t.Error("expected non-empty LastActiveTab")
	}

	// ActiveProfile should be set.
	if cfg.ActiveProfile == "" {
		t.Error("expected non-empty ActiveProfile")
	}
	if cfg.ActiveProfile != "default" {
		t.Errorf("expected ActiveProfile=default, got %s", cfg.ActiveProfile)
	}
}

func TestSaveAndLoadConfig_RoundTrip(t *testing.T) {
	// Redirect config directory to a temp dir by setting XDG_CONFIG_HOME.
	tmpDir := t.TempDir()
	originalXDG := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Cleanup(func() {
		if originalXDG == "" {
			os.Unsetenv("XDG_CONFIG_HOME")
		} else {
			os.Setenv("XDG_CONFIG_HOME", originalXDG)
		}
	})

	// Create a config with custom values.
	cfg := &Config{
		ProcessWhitelist: []string{"explorer.exe", "svchost.exe"},
		DefaultCleanOptions: defaultCleanOptionsForTest(),
		RAMMonitor: RAMMonitorSettings{
			FreeThresholdPercent:    25.0,
			StandbyThresholdPercent: 60.0,
		},
		UIPreferences: UIPreferences{
			LastActiveTab: "cleaner",
		},
		ActiveProfile: "gaming",
	}

	// Save.
	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	// Load.
	loaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify fields match.
	if len(loaded.ProcessWhitelist) != 2 {
		t.Errorf("expected 2 whitelist entries, got %d", len(loaded.ProcessWhitelist))
	}
	if loaded.ProcessWhitelist[0] != "explorer.exe" {
		t.Errorf("expected first whitelist entry=explorer.exe, got %s", loaded.ProcessWhitelist[0])
	}
	if loaded.ProcessWhitelist[1] != "svchost.exe" {
		t.Errorf("expected second whitelist entry=svchost.exe, got %s", loaded.ProcessWhitelist[1])
	}

	if loaded.ActiveProfile != "gaming" {
		t.Errorf("expected ActiveProfile=gaming, got %s", loaded.ActiveProfile)
	}
	if loaded.UIPreferences.LastActiveTab != "cleaner" {
		t.Errorf("expected LastActiveTab=cleaner, got %s", loaded.UIPreferences.LastActiveTab)
	}
	if loaded.RAMMonitor.FreeThresholdPercent != 25.0 {
		t.Errorf("expected FreeThresholdPercent=25.0, got %f", loaded.RAMMonitor.FreeThresholdPercent)
	}
	if loaded.RAMMonitor.StandbyThresholdPercent != 60.0 {
		t.Errorf("expected StandbyThresholdPercent=60.0, got %f", loaded.RAMMonitor.StandbyThresholdPercent)
	}

	// Verify clean options survived the round-trip.
	if !loaded.DefaultCleanOptions.SteamCache {
		t.Error("expected SteamCache=true after round-trip")
	}
	if !loaded.DefaultCleanOptions.DryRun {
		t.Error("expected DryRun=true after round-trip")
	}
	if loaded.DefaultCleanOptions.RecycleBin {
		t.Error("expected RecycleBin=false after round-trip")
	}
}

func TestLoadConfig_ReturnsDefaultWhenNoFileExists(t *testing.T) {
	// Point config dir to an empty temp directory so no config file exists.
	tmpDir := t.TempDir()
	originalXDG := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Cleanup(func() {
		if originalXDG == "" {
			os.Unsetenv("XDG_CONFIG_HOME")
		} else {
			os.Setenv("XDG_CONFIG_HOME", originalXDG)
		}
	})

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig with no file should not error, got: %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadConfig should return a non-nil default config")
	}

	// Compare against DefaultConfig to make sure they match.
	def := DefaultConfig()
	if cfg.ActiveProfile != def.ActiveProfile {
		t.Errorf("expected ActiveProfile=%s, got %s", def.ActiveProfile, cfg.ActiveProfile)
	}
	if cfg.DefaultCleanOptions.WindowsTemp != def.DefaultCleanOptions.WindowsTemp {
		t.Error("loaded config WindowsTemp does not match default")
	}
	if cfg.RAMMonitor.FreeThresholdPercent != def.RAMMonitor.FreeThresholdPercent {
		t.Error("loaded config FreeThresholdPercent does not match default")
	}
}

// defaultCleanOptionsForTest returns a CleanOptions with a mix of enabled fields
// for testing serialization round-trips.
func defaultCleanOptionsForTest() cleaner.CleanOptions {
	return cleaner.CleanOptions{
		WindowsTemp: true,
		UserTemp:    true,
		SteamCache:  true,
		DryRun:      true,
	}
}
