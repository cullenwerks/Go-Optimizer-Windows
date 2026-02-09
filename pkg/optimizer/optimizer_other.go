//go:build !windows

package optimizer

import "syscall"

func getSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{}
}

func optimizeStartupPlatform() StartupResult {
	return StartupResult{}
}

func setNetworkThrottling() error {
	return nil
}
