package scanner

import (
	"context"

	"echo-rebuild/internal/store"
)

type Scanner interface {
	Scan(ctx context.Context, opts ScanOptions) ([]store.AppEntry, error)
}

type ScanOptions struct {
	Platform string
}

var newScanner func() Scanner

func New() Scanner {
	if newScanner == nil {
		return &fallbackScanner{}
	}
	return newScanner()
}

type fallbackScanner struct{}

func (s *fallbackScanner) Scan(_ context.Context, _ ScanOptions) ([]store.AppEntry, error) {
	return []store.AppEntry{}, nil
}
