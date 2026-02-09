//go:build windows

package optimizer

import (
	"syscall"

	"golang.org/x/sys/windows/registry"
)

var unnecessaryStartup = []string{
	"OneDrive", "Skype", "Spotify", "Discord",
	"Steam", "EpicGamesLauncher", "AdobeUpdater",
	"iTunes", "iTunesHelper",
}

func getSysProcAttr() *syscall.SysProcAttr {
	// NOTE: Do NOT set HideWindow: true — it triggers AV heuristics
	// (Trojan:Win32/Bearfoos.B!ml) because hidden child processes are
	// a common malware pattern.
	return &syscall.SysProcAttr{}
}

func optimizeStartupPlatform() StartupResult {
	result := StartupResult{}

	regPaths := []struct {
		root registry.Key
		path string
	}{
		{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`},
		{registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`},
	}

	for _, rp := range regPaths {
		key, err := registry.OpenKey(rp.root, rp.path, registry.QUERY_VALUE|registry.SET_VALUE)
		if err != nil {
			continue
		}

		names, err := key.ReadValueNames(-1)
		if err != nil {
			key.Close()
			continue
		}

		for _, name := range names {
			val, _, err := key.GetStringValue(name)
			if err != nil {
				continue
			}

			isUnnecessary := false
			for _, u := range unnecessaryStartup {
				if name == u {
					isUnnecessary = true
					break
				}
			}

			prog := StartupProgram{
				Name: name,
				Path: val,
			}

			if isUnnecessary {
				prog.Impact = "High"
				if err := key.DeleteValue(name); err == nil {
					prog.Disabled = true
					result.Disabled++
				}
			} else {
				prog.Impact = "Low"
			}
			result.Programs = append(result.Programs, prog)
		}
		key.Close()
	}

	return result
}

func setNetworkThrottling() error {
	key, _, err := registry.CreateKey(registry.LOCAL_MACHINE,
		`SOFTWARE\Microsoft\Windows NT\CurrentVersion\Multimedia\SystemProfile`,
		registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	return key.SetDWordValue("NetworkThrottlingIndex", 0xffffffff)
}
