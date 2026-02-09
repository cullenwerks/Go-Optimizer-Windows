package gaming

import (
	"testing"
)

// ---------- GetGameProfile tests ----------

func TestGetGameProfile_ValidName(t *testing.T) {
	profile := GetGameProfile("Valorant")
	if profile == nil {
		t.Fatal("expected non-nil profile for Valorant")
	}
	if profile.Name != "Valorant" {
		t.Errorf("expected Name=Valorant, got %s", profile.Name)
	}
	if profile.CPUPriority != "High" {
		t.Errorf("expected CPUPriority=High, got %s", profile.CPUPriority)
	}
}

func TestGetGameProfile_CaseInsensitive(t *testing.T) {
	profile := GetGameProfile("league of legends")
	if profile == nil {
		t.Fatal("expected non-nil profile for case-insensitive lookup")
	}
	if profile.Name != "League of Legends" {
		t.Errorf("expected Name=League of Legends, got %s", profile.Name)
	}
}

func TestGetGameProfile_InvalidName(t *testing.T) {
	profile := GetGameProfile("NonexistentGame123")
	if profile != nil {
		t.Errorf("expected nil for unknown game, got %+v", profile)
	}
}

func TestGetGameProfile_EmptyName(t *testing.T) {
	profile := GetGameProfile("")
	if profile != nil {
		t.Errorf("expected nil for empty name, got %+v", profile)
	}
}

// ---------- GetGameProfileByExe tests ----------

func TestGetGameProfileByExe_ValidExe(t *testing.T) {
	profile := GetGameProfileByExe("cs2.exe")
	if profile == nil {
		t.Fatal("expected non-nil profile for cs2.exe")
	}
	if profile.Name != "CS2" {
		t.Errorf("expected Name=CS2, got %s", profile.Name)
	}
}

func TestGetGameProfileByExe_CaseInsensitive(t *testing.T) {
	tests := []struct {
		exe          string
		expectedGame string
	}{
		{"VALORANT.EXE", "Valorant"},
		{"valorant.exe", "Valorant"},
		{"Valorant.Exe", "Valorant"},
		{"CS2.EXE", "CS2"},
		{"leagueclient.exe", "League of Legends"},
		{"R5APEX.EXE", "Apex Legends"},
	}

	for _, tc := range tests {
		t.Run(tc.exe, func(t *testing.T) {
			profile := GetGameProfileByExe(tc.exe)
			if profile == nil {
				t.Fatalf("expected non-nil profile for %s", tc.exe)
			}
			if profile.Name != tc.expectedGame {
				t.Errorf("expected Name=%s, got %s", tc.expectedGame, profile.Name)
			}
		})
	}
}

func TestGetGameProfileByExe_InvalidExe(t *testing.T) {
	profile := GetGameProfileByExe("notepad.exe")
	if profile != nil {
		t.Errorf("expected nil for unknown exe, got %+v", profile)
	}
}

func TestGetGameProfileByExe_EmptyExe(t *testing.T) {
	profile := GetGameProfileByExe("")
	if profile != nil {
		t.Errorf("expected nil for empty exe, got %+v", profile)
	}
}

// ---------- GameProfile field validation ----------

func TestPredefinedGames_HaveRequiredFields(t *testing.T) {
	for _, g := range PredefinedGames {
		if g.Name == "" {
			t.Error("found game profile with empty Name")
		}
		if len(g.Executables) == 0 {
			t.Errorf("game %s has no executables", g.Name)
		}
		if g.CPUPriority == "" {
			t.Errorf("game %s has no CPUPriority", g.Name)
		}
	}
}

func TestGetGameProfileByExe_ReturnsPointerToOriginal(t *testing.T) {
	// Ensure the returned pointer refers to the slice element, not a copy.
	profile := GetGameProfileByExe("cs2.exe")
	if profile == nil {
		t.Fatal("expected non-nil profile")
	}
	found := false
	for i := range PredefinedGames {
		if &PredefinedGames[i] == profile {
			found = true
			break
		}
	}
	if !found {
		t.Error("returned pointer does not reference an element of PredefinedGames")
	}
}
