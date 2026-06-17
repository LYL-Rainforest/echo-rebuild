//go:build linux || darwin || freebsd

package scanner

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"echo-rebuild/internal/store"
)

func init() {
	newScanner = func() Scanner { return &unixScanner{} }
}

type unixScanner struct{}

func (s *unixScanner) Scan(ctx context.Context, _ ScanOptions) ([]store.AppEntry, error) {
	var entries []store.AppEntry

	switch {
	case isCommandAvailable("dpkg-query"):
		entries = s.scanDpkg(ctx)
	case isCommandAvailable("brew"):
		entries = s.scanBrew(ctx)
	case isCommandAvailable("pkg"):
		entries = s.scanPkg(ctx)
	}

	return entries, nil
}

func (s *unixScanner) scanDpkg(ctx context.Context) []store.AppEntry {
	cmd := exec.CommandContext(ctx, "dpkg-query", "-W", "-f", `${Package}\t${Version}\n`)
	output, err := cmd.Output()
	if err != nil {
		return nil
	}
	var entries []store.AppEntry
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		name := parts[0]
		entries = append(entries, store.AppEntry{
			Name:     fmt.Sprintf("[软件] %s", name),
			Platform: "linux",
		})
	}
	return entries
}

func (s *unixScanner) scanBrew(ctx context.Context) []store.AppEntry {
	cmd := exec.CommandContext(ctx, "brew", "list", "--cask", "--versions")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}
	var entries []store.AppEntry
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name := strings.Fields(line)[0]
		entries = append(entries, store.AppEntry{
			Name:     fmt.Sprintf("[软件] %s", name),
			Platform: "darwin",
		})
	}
	return entries
}

func (s *unixScanner) scanPkg(ctx context.Context) []store.AppEntry {
	cmd := exec.CommandContext(ctx, "pkg", "info")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}
	var entries []store.AppEntry
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name := strings.Fields(line)[0]
		entries = append(entries, store.AppEntry{
			Name:     fmt.Sprintf("[软件] %s", name),
			Platform: "freebsd",
		})
	}
	return entries
}

func isCommandAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
