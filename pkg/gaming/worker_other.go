//go:build !windows

package gaming

import "fmt"

// RunExtremeModeWorker is a stub on non-Windows platforms.
func RunExtremeModeWorker(action string, progress func(string)) error {
	return fmt.Errorf("extreme mode subprocess only supported on Windows")
}
