package app

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"echo-rebuild/internal/engine"
	"echo-rebuild/internal/scanner"
	"echo-rebuild/internal/store"
	"echo-rebuild/internal/tbi"
)

type RestoreSummary struct {
	Success  int
	Manual   int
	Fallback int
	Skipped  int
}

type Workflow struct {
	db           *sql.DB
	scanner      scanner.Scanner
	installer    *engine.Installer
	pool         *engine.WorkerPool
	imageManager *tbi.ImageManager
}

func NewWorkflow(dbPath string) (*Workflow, error) {
	db, err := store.InitDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("init db: %w", err)
	}

	return &Workflow{
		db:           db,
		scanner:      scanner.New(),
		installer:    engine.NewInstaller(""),
		pool:         engine.NewPool(0),
		imageManager: tbi.NewImageManager(),
	}, nil
}

func (w *Workflow) DB() *sql.DB {
	return w.db
}

func (w *Workflow) Scanner() scanner.Scanner {
	return w.scanner
}

func (w *Workflow) BackupConfig(ctx context.Context, entries []store.AppEntry) error {
	if len(entries) == 0 {
		return fmt.Errorf("no entries to backup")
	}
	return store.SaveEntries(w.db, entries)
}

func (w *Workflow) RestoreConfig(ctx context.Context, entries []store.AppEntry, restoreBaseDir string) *RestoreSummary {
	summary := &RestoreSummary{}
	var mu sync.Mutex

	addResult := func(info string) {
		mu.Lock()
		defer mu.Unlock()
		switch info {
		case "success":
			summary.Success++
		case "manual":
			summary.Manual++
		case "fallback":
			summary.Fallback++
		default:
			summary.Skipped++
		}
	}

	jobs, results := w.pool.Start(ctx, func(ctx context.Context, data any) engine.Result {
		entry := data.(store.AppEntry)
		res := engine.Result{}

		switch {
		case entry.IsArchive:
			if err := w.installer.OpenArchive(entry, restoreBaseDir); err != nil {
				res.Err = err
				res.Info = "skipped"
			} else {
				res.Info = "manual"
			}

		case entry.PackagePath != "":
			if err := w.installer.CopyPortable(ctx, entry, restoreBaseDir); err != nil {
				res.Err = err
				res.Info = "skipped"
			} else {
				res.Info = "success"
			}

		case entry.DownloadURL != "":
			if err := w.installer.DownloadAndRun(ctx, entry); err != nil {
				if openErr := w.installer.OpenURL(entry); openErr != nil {
					res.Err = fmt.Errorf("install: %v; open url: %v", err, openErr)
				}
				res.Info = "fallback"
			} else {
				res.Info = "success"
			}

		default:
			res.Info = "skipped"
		}

		return res
	})

	go func() {
		for _, entry := range entries {
			jobs <- engine.Job{Data: entry}
		}
		close(jobs)
	}()

	for res := range results {
		addResult(res.Info)
	}

	w.pool.Wait()
	return summary
}

func (w *Workflow) CaptureImage(ctx context.Context, source, output string, imgType tbi.ImageType, opts tbi.CaptureOptions) error {
	return w.imageManager.Capture(ctx, source, output, imgType, opts)
}

func (w *Workflow) RestoreImage(ctx context.Context, image, target string, opts tbi.RestoreOptions) error {
	return w.imageManager.Restore(ctx, image, target, opts)
}
