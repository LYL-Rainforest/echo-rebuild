package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"echo-rebuild/internal/store"
)

func tmpDir(t *testing.T) string {
	t.Helper()
	d, err := os.MkdirTemp("", "echo-wf-*")
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { os.RemoveAll(d) })
	return d
}

func TestNewWorkflow_Valid(t *testing.T) {
	w, err := NewWorkflow(filepath.Join(tmpDir(t), "t.db"))
	if err != nil { t.Fatal(err) }
	defer w.DB().Close()
	if w.DB() == nil { t.Fatal("DB is nil") }
}

func TestNewWorkflow_InvalidDir(t *testing.T) {
	_, err := NewWorkflow("/nonexistent/deep/dir/db.db")
	if err == nil { t.Fatal("expected error") }
}

func TestBackupConfig_Valid(t *testing.T) {
	w, _ := NewWorkflow(filepath.Join(tmpDir(t), "b.db"))
	defer w.DB().Close()
	err := w.BackupConfig(context.Background(), []store.AppEntry{
		{Name:"A", Platform:"windows"}, {Name:"B", Platform:"linux"},
	})
	if err != nil { t.Fatal(err) }
}

func TestBackupConfig_Empty(t *testing.T) {
	w, _ := NewWorkflow(filepath.Join(tmpDir(t), "e.db"))
	defer w.DB().Close()
	err := w.BackupConfig(context.Background(), []store.AppEntry{})
	if err == nil { t.Fatal("expected error for empty") }
}

func TestBackupConfig_Nil(t *testing.T) {
	w, _ := NewWorkflow(filepath.Join(tmpDir(t), "n.db"))
	defer w.DB().Close()
	err := w.BackupConfig(context.Background(), nil)
	if err == nil { t.Fatal("expected error for nil") }
}

func TestBackupConfig_InvalidEntry(t *testing.T) {
	w, _ := NewWorkflow(filepath.Join(tmpDir(t), "i.db"))
	defer w.DB().Close()
	err := w.BackupConfig(context.Background(), []store.AppEntry{{}})
	if err == nil { t.Fatal("expected error") }
}

func TestRestoreConfig_Empty(t *testing.T) {
	w, _ := NewWorkflow(filepath.Join(tmpDir(t), "re.db"))
	defer w.DB().Close()
	s := w.RestoreConfig(context.Background(), []store.AppEntry{}, tmpDir(t))
	if s.Success!=0 || s.Manual!=0 || s.Fallback!=0 || s.Skipped!=0 { t.Fatalf("%+v", s) }
}

func TestRestoreConfig_Skipped(t *testing.T) {
	w, _ := NewWorkflow(filepath.Join(tmpDir(t), "sk.db"))
	defer w.DB().Close()
	s := w.RestoreConfig(context.Background(), []store.AppEntry{
		{Name:"A"},{Name:"B"},{Name:"C"},
	}, tmpDir(t))
	if s.Skipped != 3 || len(s.SkippedNames) != 3 { t.Fatalf("expected 3 skipped, got %+v", s) }
}

func TestRestoreConfig_ArchiveFails(t *testing.T) {
	w, _ := NewWorkflow(filepath.Join(tmpDir(t), "ar.db"))
	defer w.DB().Close()
	// non-existent restoreBaseDir → OpenArchive fails before opening explorer
	s := w.RestoreConfig(context.Background(), []store.AppEntry{
		{Name:"Arc", IsArchive:true, PackagePath:"x.7z"},
	}, "/nonexistent-restore")
	t.Logf("archive result: %+v", s)
}

func TestRestoreConfig_Mixed(t *testing.T) {
	dir := tmpDir(t)
	w, _ := NewWorkflow(filepath.Join(dir, "mx.db"))
	defer w.DB().Close()
	entries := []store.AppEntry{
		{Name:"S1"}, {Name:"S2"},
		{Name:"Arc1", IsArchive:true, PackagePath:"x.7z"},
	}
	s := w.RestoreConfig(context.Background(), entries, "/nonexistent-restore")
	// all 3 have no valid source → all skipped
	if s.Skipped != 3 || len(s.SkippedNames) != 3 { t.Fatalf("skipped: %v", s) }
}


