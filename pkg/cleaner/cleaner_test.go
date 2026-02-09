package cleaner

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// helper: createTempFiles creates n files in dir and returns their paths.
func createTempFiles(t *testing.T, dir string, n int) []string {
	t.Helper()
	paths := make([]string, 0, n)
	for i := 0; i < n; i++ {
		f, err := os.CreateTemp(dir, "testfile-*.tmp")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		// Write some data so SpaceFreed is non-zero.
		if _, err := f.WriteString("test data content"); err != nil {
			t.Fatalf("failed to write temp file: %v", err)
		}
		paths = append(paths, f.Name())
		f.Close()
	}
	return paths
}

// ---------- cleanDirectory tests ----------

func TestCleanDirectory_DeletesFiles(t *testing.T) {
	dir := t.TempDir()
	files := createTempFiles(t, dir, 3)

	result := cleanDirectory(dir, 0, false)

	if result.FilesDeleted != 3 {
		t.Errorf("expected 3 files deleted, got %d", result.FilesDeleted)
	}
	if result.SpaceFreed <= 0 {
		t.Errorf("expected SpaceFreed > 0, got %d", result.SpaceFreed)
	}

	// Verify that files no longer exist on disk.
	for _, f := range files {
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Errorf("file %s should have been deleted but still exists", f)
		}
	}
}

func TestCleanDirectory_DryRun(t *testing.T) {
	dir := t.TempDir()
	files := createTempFiles(t, dir, 4)

	result := cleanDirectory(dir, 0, true)

	if result.FilesDeleted != 4 {
		t.Errorf("expected 4 files reported as deleted in dry-run, got %d", result.FilesDeleted)
	}
	if result.SpaceFreed <= 0 {
		t.Errorf("expected SpaceFreed > 0 in dry-run, got %d", result.SpaceFreed)
	}

	// Verify that files still exist (dry-run should not actually remove them).
	for _, f := range files {
		if _, err := os.Stat(f); err != nil {
			t.Errorf("file %s should still exist in dry-run mode but got error: %v", f, err)
		}
	}
}

func TestCleanDirectory_AgeFiltering(t *testing.T) {
	dir := t.TempDir()
	files := createTempFiles(t, dir, 2)

	// Use a very large maxAge so that the freshly-created files are too new.
	result := cleanDirectory(dir, 24*365*time.Hour, false)

	if result.FilesDeleted != 0 {
		t.Errorf("expected 0 files deleted with large maxAge, got %d", result.FilesDeleted)
	}

	// Files should still exist because they are newer than the threshold.
	for _, f := range files {
		if _, err := os.Stat(f); err != nil {
			t.Errorf("file %s should still exist (too new for maxAge) but got error: %v", f, err)
		}
	}
}

func TestCleanDirectory_NonexistentDir(t *testing.T) {
	result := cleanDirectory(filepath.Join(t.TempDir(), "nonexistent"), 0, false)

	if result.FilesDeleted != 0 {
		t.Errorf("expected 0 files deleted for nonexistent dir, got %d", result.FilesDeleted)
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected no errors for nonexistent dir, got %v", result.Errors)
	}
}

func TestCleanDirectory_SubdirFiles(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "subdir")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	createTempFiles(t, sub, 2)
	createTempFiles(t, dir, 1)

	result := cleanDirectory(dir, 0, false)

	if result.FilesDeleted != 3 {
		t.Errorf("expected 3 files deleted (including subdir), got %d", result.FilesDeleted)
	}
}

// ---------- classifyError tests ----------

func TestClassifyError_PermissionDenied(t *testing.T) {
	err := os.ErrPermission
	ce := classifyError("/some/path", err)

	if ce.Type != ErrorPermissionDenied {
		t.Errorf("expected ErrorPermissionDenied, got %d", ce.Type)
	}
	if ce.Path != "/some/path" {
		t.Errorf("expected path /some/path, got %s", ce.Path)
	}
}

func TestClassifyError_NotExist(t *testing.T) {
	err := os.ErrNotExist
	ce := classifyError("/missing/file", err)

	if ce.Type != ErrorNotFound {
		t.Errorf("expected ErrorNotFound, got %d", ce.Type)
	}
}

func TestClassifyError_Locked(t *testing.T) {
	err := errors.New("the file is used by another process")
	ce := classifyError("/locked/file", err)

	if ce.Type != ErrorLocked {
		t.Errorf("expected ErrorLocked, got %d", ce.Type)
	}
}

func TestClassifyError_LockedSharingViolation(t *testing.T) {
	err := errors.New("sharing violation on resource")
	ce := classifyError("/locked/file2", err)

	if ce.Type != ErrorLocked {
		t.Errorf("expected ErrorLocked for sharing violation, got %d", ce.Type)
	}
}

func TestClassifyError_Timeout(t *testing.T) {
	err := errors.New("operation timeout")
	ce := classifyError("/slow/file", err)

	if ce.Type != ErrorTimeout {
		t.Errorf("expected ErrorTimeout, got %d", ce.Type)
	}
}

func TestClassifyError_Other(t *testing.T) {
	err := errors.New("some random failure")
	ce := classifyError("/other/file", err)

	if ce.Type != ErrorOther {
		t.Errorf("expected ErrorOther, got %d", ce.Type)
	}
}

func TestCleanError_ErrorString(t *testing.T) {
	ce := &CleanError{
		Path: "/test/path",
		Type: ErrorOther,
		Err:  errors.New("boom"),
	}
	expected := "/test/path: boom"
	if ce.Error() != expected {
		t.Errorf("expected %q, got %q", expected, ce.Error())
	}
}

// ---------- FormatBytes tests ----------

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.00 KB"},
		{1536, "1.50 KB"},
		{1048576, "1.00 MB"},
		{1073741824, "1.00 GB"},
	}

	for _, tc := range tests {
		got := FormatBytes(tc.input)
		if got != tc.expected {
			t.Errorf("FormatBytes(%d) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

// ---------- CleanResult merge tests ----------

func TestCleanResult_Merge(t *testing.T) {
	a := CleanResult{
		FilesDeleted:    5,
		SkippedFiles:    1,
		SpaceFreed:      1000,
		LockedFiles:     1,
		PermissionFiles: 0,
		Errors:          []error{errors.New("err1")},
	}
	b := CleanResult{
		FilesDeleted:    3,
		SkippedFiles:    2,
		SpaceFreed:      500,
		LockedFiles:     0,
		PermissionFiles: 1,
		Errors:          []error{errors.New("err2")},
	}

	a.merge(b)

	if a.FilesDeleted != 8 {
		t.Errorf("expected FilesDeleted=8, got %d", a.FilesDeleted)
	}
	if a.SkippedFiles != 3 {
		t.Errorf("expected SkippedFiles=3, got %d", a.SkippedFiles)
	}
	if a.SpaceFreed != 1500 {
		t.Errorf("expected SpaceFreed=1500, got %d", a.SpaceFreed)
	}
	if a.LockedFiles != 1 {
		t.Errorf("expected LockedFiles=1, got %d", a.LockedFiles)
	}
	if a.PermissionFiles != 1 {
		t.Errorf("expected PermissionFiles=1, got %d", a.PermissionFiles)
	}
	if len(a.Errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(a.Errors))
	}
}
