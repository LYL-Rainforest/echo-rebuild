package msg

import "echo-rebuild/internal/store"

type MenuChoice int

const (
	MenuConfigBackup MenuChoice = iota + 1
	MenuConfigRestore
	MenuExit
)

type ScanDone struct {
	Entries []store.AppEntry
	Err     error
}

type SaveDone struct {
	Path  string
	Count int
	Err   error
}

type SearchDone struct {
	URLs []string
	Err  error
}

type RestoreDone struct {
	Summary any
}
