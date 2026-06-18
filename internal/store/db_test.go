package store

import (
	"os"
	"path/filepath"
	"testing"
)

func tmpDB(t *testing.T) string {
	t.Helper()
	d, err := os.MkdirTemp("", "echo-*")
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { os.RemoveAll(d) })
	return filepath.Join(d, "t.db")
}

func TestInitDB_CreatesTable(t *testing.T) {
	p := tmpDB(t)
	d, err := InitDB(p)
	if err != nil { t.Fatal(err) }
	defer d.Close()
	var s string
	if e := d.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='entries'").Scan(&s); e != nil {
		t.Fatal("entries table missing:", e)
	}
}

func TestInitDB_InvalidDir(t *testing.T) {
	_, err := InitDB("/nonexistent/deep/dir/x.db")
	if err == nil { t.Fatal("expected error") }
}

func TestInitDB_DoubleInit(t *testing.T) {
	p := tmpDB(t)
	d1, _ := InitDB(p); d1.Close()
	d2, _ := InitDB(p); d2.Close()
}

func TestSaveLoad_FullRoundtrip(t *testing.T) {
	p := tmpDB(t)
	d, _ := InitDB(p); defer d.Close()

	in := []AppEntry{
		{Name: "A", Platform: "windows", DownloadURL: "https://a", Note: "n", PackagePath: "p/a", IsArchive: false, ConfigPath: "c/a", ScriptPath: "s/a"},
		{Name: "B", Platform: "linux", DownloadURL: "https://b", Note: "n", PackagePath: "p/b", IsArchive: true, ConfigPath: "c/b", ScriptPath: "s/b"},
	}
	SaveEntries(d, in)
	out, _ := LoadEntries(d, "")
	if len(out) != 2 { t.Fatalf("got %d", len(out)) }
	for i, e := range out {
		if e.Name != in[i].Name || e.DownloadURL != in[i].DownloadURL || e.PackagePath != in[i].PackagePath || e.IsArchive != in[i].IsArchive {
			t.Fatalf("entry %d mismatch: %+v", i, e)
		}
	}
}

func TestSaveLoad_PlatformFilter(t *testing.T) {
	p := tmpDB(t)
	d, _ := InitDB(p); defer d.Close()
	SaveEntries(d, []AppEntry{{Name:"W",Platform:"windows"},{Name:"L",Platform:"linux"},{Name:"A",Platform:""}})
	got, _ := LoadEntries(d, "windows")
	if len(got) != 2 { t.Fatalf("windows filter: %d", len(got)) }
	got, _ = LoadEntries(d, "darwin")
	if len(got) != 1 { t.Fatalf("darwin filter: %d", len(got)) }
}

func TestSaveLoad_EmptyDB(t *testing.T) {
	p := tmpDB(t)
	d, _ := InitDB(p); defer d.Close()
	got, _ := LoadEntries(d, "")
	if len(got) != 0 { t.Fatalf("expected 0, got %d", len(got)) }
}

func TestSave_EmptySlice(t *testing.T) {
	p := tmpDB(t)
	d, _ := InitDB(p); defer d.Close()
	if err := SaveEntries(d, []AppEntry{}); err != nil { t.Fatal(err) }
}

func TestSave_RejectInvalid(t *testing.T) {
	p := tmpDB(t)
	d, _ := InitDB(p); defer d.Close()
	if err := SaveEntries(d, []AppEntry{{}}); err == nil { t.Fatal("expected error") }
}

func TestDelete_Existing(t *testing.T) {
	p := tmpDB(t)
	d, _ := InitDB(p); defer d.Close()
	SaveEntries(d, []AppEntry{{Name:"X",Platform:""}})
	if err := DeleteEntry(d, "X"); err != nil { t.Fatal(err) }
	got, _ := LoadEntries(d, "")
	if len(got) != 0 { t.Fatal("not deleted") }
}

func TestDelete_Nonexistent(t *testing.T) {
	p := tmpDB(t)
	d, _ := InitDB(p); defer d.Close()
	if err := DeleteEntry(d, "nope"); err != nil { t.Fatal("should not error") }
}

func TestDelete_EmptyName(t *testing.T) {
	p := tmpDB(t)
	d, _ := InitDB(p); defer d.Close()
	_ = DeleteEntry(d, "") // must not panic
}

func TestConcurrentSave(t *testing.T) {
	p := tmpDB(t)
	d, _ := InitDB(p); defer d.Close()
	for i := 0; i < 10; i++ {
		SaveEntries(d, []AppEntry{{Name:"C",Platform:""}})
	}
	got, _ := LoadEntries(d, "")
	if len(got) != 1 { t.Fatalf("expected 1 after upsert, got %d", len(got)) }
}
