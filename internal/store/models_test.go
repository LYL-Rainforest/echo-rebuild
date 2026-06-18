package store

import "testing"

func TestValidate_AllValidPlatforms(t *testing.T) {
	for _, p := range []string{"windows", "linux", "darwin", "freebsd", ""} {
		e := AppEntry{Name: "Test", Platform: p}
		if err := e.Validate(); err != nil {
			t.Errorf("platform %q should be valid: %v", p, err)
		}
	}
}

func TestValidate_EmptyName(t *testing.T) {
	if err := (AppEntry{}).Validate(); err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestValidate_InvalidPlatform(t *testing.T) {
	for _, p := range []string{"solaris", "aix", "android"} {
		e := AppEntry{Name: "X", Platform: p}
		if err := e.Validate(); err == nil {
			t.Errorf("platform %q should be invalid", p)
		}
	}
}

func TestValidate_ZeroValue(t *testing.T) {
	var e AppEntry
	if err := e.Validate(); err == nil {
		t.Fatal("expected error for zero value")
	}
}

func TestValidate_AllFieldsValid(t *testing.T) {
	e := AppEntry{
		Name: "FF", DownloadURL: "https://dl", NeedManualDL: true,
		Note: "note", PackagePath: "pkg/ff", IsArchive: true,
		ConfigPath: "%appdata%/ff", Platform: "windows", ScriptPath: "s.ps1",
	}
	if err := e.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestCategoryConstants(t *testing.T) {
	if CatSoftware != "software" { t.Fatalf("CatSoftware=%q", CatSoftware) }
	if CatSystem != "system" { t.Fatalf("CatSystem=%q", CatSystem) }
}
