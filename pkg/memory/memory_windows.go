//go:build windows

package memory

import (
	"fmt"
	"log"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/shirou/gopsutil/v3/mem"
	"golang.org/x/sys/windows"
)

// Memory list commands for NtSetSystemInformation
const (
	SystemMemoryListInformation       = 80
	MemoryEmptyWorkingSets            = 0
	MemoryFlushModifiedList           = 1
	MemoryPurgeStandbyList            = 2
	MemoryPurgeLowPriorityStandbyList = 3
	MemoryCombinePageLists            = 4
)

const (
	SE_PROF_SINGLE_PROCESS_PRIVILEGE = "SeProfileSingleProcessPrivilege"
	SE_INCREASE_QUOTA_PRIVILEGE      = "SeIncreaseQuotaPrivilege"
)

var (
	ntdll                      = windows.NewLazySystemDLL("ntdll.dll")
	procNtSetSystemInformation = ntdll.NewProc("NtSetSystemInformation")
	psapi                      = windows.NewLazySystemDLL("psapi.dll")
	procEmptyWorkingSet        = psapi.NewProc("EmptyWorkingSet")
	kernel32                   = windows.NewLazySystemDLL("kernel32.dll")
	procGlobalMemoryStatusEx   = kernel32.NewProc("GlobalMemoryStatusEx")

	monitorActive bool
	monitorDone   chan struct{}
	monitorMu     sync.Mutex

	// Configurable thresholds
	FreeMemoryThresholdPercent float64 = 15.0  // Trigger cleanup when free RAM drops below this %
	StandbyThresholdPercent    float64 = 40.0  // Only clear standby if it exceeds this % of total
	MinCleanInterval           = 30 * time.Second // Don't clean more often than this
	lastCleanTime              time.Time
	trimCountTotal             int64
)

// MemoryStats holds current memory status
type MemoryStats struct {
	TotalGB        float64
	UsedGB         float64
	FreeGB         float64
	StandbyGB      float64
	UsedPercent    float64
	FreePercent    float64
	StandbyPercent float64
	LastTrimTime   time.Time
	TrimCount      int64
}

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

// getMemoryStatus calls GlobalMemoryStatusEx for accurate physical RAM figures.
// Returns (total, avail, err). avail = free + standby reclaimable (ullAvailPhys).
func getMemoryStatus() (total, avail uint64, err error) {
	var ms memoryStatusEx
	ms.dwLength = uint32(unsafe.Sizeof(ms))
	ret, _, e := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&ms)))
	if ret == 0 {
		return 0, 0, fmt.Errorf("GlobalMemoryStatusEx failed: %w", e)
	}
	return ms.ullTotalPhys, ms.ullAvailPhys, nil
}

// EnableSeProfileSingleProcessPrivilege enables the required privileges
// for memory list operations. Must be called once before any trim operations.
func EnableSeProfileSingleProcessPrivilege() error {
	var token windows.Token
	err := windows.OpenProcessToken(windows.CurrentProcess(),
		windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &token)
	if err != nil {
		return fmt.Errorf("failed to open process token: %w", err)
	}
	defer token.Close()

	// Enable SE_PROF_SINGLE_PROCESS_PRIVILEGE
	if err := enablePrivilege(token, SE_PROF_SINGLE_PROCESS_PRIVILEGE); err != nil {
		return fmt.Errorf("failed to enable SeProfileSingleProcessPrivilege: %w", err)
	}

	// Enable SE_INCREASE_QUOTA_PRIVILEGE
	if err := enablePrivilege(token, SE_INCREASE_QUOTA_PRIVILEGE); err != nil {
		return fmt.Errorf("failed to enable SeIncreaseQuotaPrivilege: %w", err)
	}

	return nil
}

func enablePrivilege(token windows.Token, privilegeName string) error {
	var luid windows.LUID
	privName, err := syscall.UTF16PtrFromString(privilegeName)
	if err != nil {
		return err
	}

	err = windows.LookupPrivilegeValue(nil, privName, &luid)
	if err != nil {
		return err
	}

	tp := windows.Tokenprivileges{
		PrivilegeCount: 1,
		Privileges: [1]windows.LUIDAndAttributes{
			{
				Luid:       luid,
				Attributes: windows.SE_PRIVILEGE_ENABLED,
			},
		},
	}

	err = windows.AdjustTokenPrivileges(token, false, &tp, 0, nil, nil)
	if err != nil {
		return err
	}

	return nil
}

// ntStatusError converts a non-zero NTSTATUS to a human-readable error.
func ntStatusError(ret uintptr) error {
	switch ret {
	case 0xC0000061:
		return fmt.Errorf("requires Administrator — right-click SysCleaner and run as Administrator (NTSTATUS: 0x%X)", ret)
	case 0xC0000022:
		return fmt.Errorf("access denied — run SysCleaner as Administrator (NTSTATUS: 0x%X)", ret)
	default:
		return fmt.Errorf("NTSTATUS: 0x%X", ret)
	}
}

// PurgeStandbyList clears the standby memory list (the big one for gaming).
// This is the equivalent of RAMMap's "Empty Standby List".
// Requires SeProfileSingleProcessPrivilege (Administrator).
func PurgeStandbyList() error {
	cmd := int32(MemoryPurgeStandbyList)
	ret, _, _ := procNtSetSystemInformation.Call(
		uintptr(SystemMemoryListInformation),
		uintptr(unsafe.Pointer(&cmd)),
		uintptr(unsafe.Sizeof(cmd)),
	)
	if ret != 0 {
		return fmt.Errorf("NtSetSystemInformation failed: %w", ntStatusError(ret))
	}
	return nil
}

// FlushModifiedList moves modified (dirty) pages to the standby list by writing
// them to disk. Call before PurgeStandbyList so those pages become eligible
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

// PurgeLowPriorityStandby clears only low-priority standby pages.
// This is gentler than PurgeStandbyList and less likely to cause stutter.
func PurgeLowPriorityStandby() error {
	cmd := int32(MemoryPurgeLowPriorityStandbyList)
	ret, _, _ := procNtSetSystemInformation.Call(
		uintptr(SystemMemoryListInformation),
		uintptr(unsafe.Pointer(&cmd)),
		uintptr(unsafe.Sizeof(cmd)),
	)
	if ret != 0 {
		return fmt.Errorf("NtSetSystemInformation failed: %w", ntStatusError(ret))
	}
	return nil
}

// EmptyProcessWorkingSet trims the working set of a specific process.
// This is gentler than purging the standby list.
func EmptyProcessWorkingSet(pid uint32) error {
	handle, err := windows.OpenProcess(
		windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_SET_QUOTA,
		false, pid)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)

	ret, _, err := procEmptyWorkingSet.Call(uintptr(handle))
	if ret == 0 {
		return fmt.Errorf("EmptyWorkingSet failed: %v", err)
	}
	return nil
}

// StartContinuousMonitor begins monitoring RAM and trimming standby memory
// when free RAM drops below the threshold. This is ONLY active during
// Extreme Gaming Mode.
//
// Strategy (to avoid performance drops):
//  1. Check free memory every 5 seconds
//  2. If free memory < 15% of total AND standby > 40% of total:
//     a. First attempt: PurgeLowPriorityStandby (gentle)
//     b. If still low after 10s: PurgeStandbyList (aggressive)
//  3. Never trim more often than every 30 seconds
//  4. Log every trim action with before/after stats
func StartContinuousMonitor(statsCallback func(MemoryStats)) {
	monitorMu.Lock()
	if monitorActive {
		monitorMu.Unlock()
		return
	}
	monitorDone = make(chan struct{})
	monitorActive = true
	monitorMu.Unlock()

	// Enable required privileges
	if err := EnableSeProfileSingleProcessPrivilege(); err != nil {
		log.Printf("[SysCleaner] Warning: Failed to enable memory privileges: %v", err)
		log.Println("[SysCleaner] RAM trimming may not work correctly. Run as Administrator.")
	}

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-monitorDone:
				return
			case <-ticker.C:
				// Use GlobalMemoryStatusEx for accurate free/available; fall back to gopsutil
				total, avail, statErr := getMemoryStatus()
				vmem, _ := mem.VirtualMemory()

				if statErr != nil && vmem == nil {
					continue
				}
				if statErr != nil {
					total = vmem.Total
					avail = vmem.Available
				}

				totalGB := float64(total) / 1024 / 1024 / 1024
				freeGB := float64(avail) / 1024 / 1024 / 1024
				usedGB := float64(0)
				usedPercent := float64(0)
				standbyGB := float64(0)
				if vmem != nil {
					usedGB = float64(vmem.Used) / 1024 / 1024 / 1024
					usedPercent = vmem.UsedPercent
					// Standby approximation: Available - truly-free (gopsutil approximation)
					standbyGB = freeGB - (float64(vmem.Free) / 1024 / 1024 / 1024)
					if standbyGB < 0 {
						standbyGB = 0
					}
				}

				freePercent := (freeGB / totalGB) * 100
				standbyPercent := (standbyGB / totalGB) * 100

				monitorMu.Lock()
				lastTrim := lastCleanTime
				trimCnt := trimCountTotal
				monitorMu.Unlock()

				stats := MemoryStats{
					TotalGB:        totalGB,
					UsedGB:         usedGB,
					FreeGB:         freeGB,
					StandbyGB:      standbyGB,
					UsedPercent:    usedPercent,
					FreePercent:    freePercent,
					StandbyPercent: standbyPercent,
					LastTrimTime:   lastTrim,
					TrimCount:      trimCnt,
				}

				if statsCallback != nil {
					statsCallback(stats)
				}

				// Should we trim?
				monitorMu.Lock()
				shouldTrim := freePercent < FreeMemoryThresholdPercent &&
					standbyPercent > StandbyThresholdPercent &&
					time.Since(lastCleanTime) > MinCleanInterval
				monitorMu.Unlock()

				if shouldTrim {
					log.Printf("[SysCleaner] RAM Monitor: Free=%.1f%%, Standby=%.1f%% - Trimming...",
						freePercent, standbyPercent)

					// Try gentle first
					if err := PurgeLowPriorityStandby(); err != nil {
						log.Printf("[SysCleaner] Low-priority purge failed: %v", err)
					} else {
						log.Println("[SysCleaner] Low-priority standby trim completed")
					}

					monitorMu.Lock()
					lastCleanTime = time.Now()
					trimCountTotal++
					monitorMu.Unlock()

					// Wait 5s for memory pressure to stabilize (interruptible)
					select {
					case <-monitorDone:
						return
					case <-time.After(5 * time.Second):
					}

					total2, avail2, statErr2 := getMemoryStatus()
					if statErr2 != nil && vmem != nil {
						total2 = vmem.Total
						avail2 = vmem.Available
					}
					if statErr2 == nil || vmem != nil {
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
				}
			}
		}
	}()
}

// StopContinuousMonitor stops the RAM monitor.
func StopContinuousMonitor() {
	monitorMu.Lock()
	defer monitorMu.Unlock()
	if monitorActive && monitorDone != nil {
		close(monitorDone)
		monitorActive = false
	}
}

// TrimNow immediately trims standby memory.
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

// GetCurrentStats returns current memory statistics.
func GetCurrentStats() MemoryStats {
	total, avail, statErr := getMemoryStatus()
	vmem, _ := mem.VirtualMemory()

	if statErr != nil && vmem == nil {
		return MemoryStats{}
	}
	if statErr != nil {
		total = vmem.Total
		avail = vmem.Available
	}

	totalGB := float64(total) / 1024 / 1024 / 1024
	freeGB := float64(avail) / 1024 / 1024 / 1024
	usedGB := float64(0)
	usedPercent := float64(0)
	standbyGB := float64(0)
	if vmem != nil {
		usedGB = float64(vmem.Used) / 1024 / 1024 / 1024
		usedPercent = vmem.UsedPercent
		standbyGB = freeGB - (float64(vmem.Free) / 1024 / 1024 / 1024)
		if standbyGB < 0 {
			standbyGB = 0
		}
	}

	freePercent := (freeGB / totalGB) * 100
	standbyPercent := (standbyGB / totalGB) * 100

	monitorMu.Lock()
	lastTrim := lastCleanTime
	trimCnt := trimCountTotal
	monitorMu.Unlock()

	return MemoryStats{
		TotalGB:        totalGB,
		UsedGB:         usedGB,
		FreeGB:         freeGB,
		StandbyGB:      standbyGB,
		UsedPercent:    usedPercent,
		FreePercent:    freePercent,
		StandbyPercent: standbyPercent,
		LastTrimTime:   lastTrim,
		TrimCount:      trimCnt,
	}
}
