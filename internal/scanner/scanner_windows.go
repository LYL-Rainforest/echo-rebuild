//go:build windows

package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"echo-rebuild/internal/store"
)

func init() {
	newScanner = func() Scanner { return &windowsScanner{} }
}

type windowsScanner struct{}

type psSoftware struct {
	Name            string `json:"DisplayName"`
	InstallLocation string `json:"InstallLocation"`
	Publisher       string `json:"Publisher"`
}

func (s *windowsScanner) Scan(ctx context.Context, _ ScanOptions) ([]store.AppEntry, error) {
	var entries []store.AppEntry

	sw, err := s.scanInstalledSoftware(ctx)
	if err != nil {
		return nil, fmt.Errorf("scan software: %w", err)
	}
	entries = append(entries, sw...)

	sysCtx, sysCancel := context.WithCancel(ctx)
	defer sysCancel()
	sys, _ := s.scanSystemSettings(sysCtx)
	entries = append(entries, sys...)

	return entries, nil
}

func (s *windowsScanner) scanInstalledSoftware(ctx context.Context) ([]store.AppEntry, error) {
	psScript := `
$paths = @(
	'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\*',
	'HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*',
	'HKCU:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\*'
)
$result = @()
foreach ($p in $paths) {
	$items = Get-ItemProperty -Path $p -ErrorAction SilentlyContinue
	foreach ($item in $items) {
		if ($item.DisplayName) {
			$result += @{
				DisplayName      = $item.DisplayName
				InstallLocation  = $item.InstallLocation
				Publisher        = $item.Publisher
			}
		}
	}
}
$result | ConvertTo-Json -Compress
`
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", psScript)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("powershell exec: %w", err)
	}

	var raw []psSoftware
	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}

	seen := make(map[string]bool)
	var entries []store.AppEntry
	for _, s := range raw {
		n := strings.TrimSpace(s.Name)
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		e := store.AppEntry{
			Name:     fmt.Sprintf("[软件] %s", n),
			Platform: "windows",
		}
		if s.InstallLocation != "" {
			e.ConfigPath = s.InstallLocation
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func (s *windowsScanner) scanSystemSettings(ctx context.Context) ([]store.AppEntry, error) {
	tmpDir, err := os.MkdirTemp("", "echorebuild-sys-*")
	if err != nil {
		return nil, fmt.Errorf("temp dir: %w", err)
	}

	regKeys := []struct {
		Name     string
		RegPath  string
	}{
		{"当前用户环境变量", "HKCU\\Environment"},
		{"系统环境变量", "HKLM\\SYSTEM\\CurrentControlSet\\Control\\Session Manager\\Environment"},
		{"当前用户注册表", "HKCU\\Software"},
		{"网络连接配置", "HKLM\\SYSTEM\\CurrentControlSet\\Services\\Tcpip\\Parameters"},
		{"电源方案", "HKLM\\SYSTEM\\CurrentControlSet\\Control\\Power\\UserPowerSchemes"},
	}

	var entries []store.AppEntry
	for _, rk := range regKeys {
		select {
		case <-ctx.Done():
			return entries, ctx.Err()
		default:
		}

		regFile := filepath.Join(tmpDir, fmt.Sprintf("%s.reg", strings.ReplaceAll(rk.Name, " ", "_")))
		cmd := exec.CommandContext(ctx, "reg", "export", rk.RegPath, regFile, "/y")
		if err := cmd.Run(); err != nil {
			continue
		}

		entries = append(entries, store.AppEntry{
			Name:    fmt.Sprintf("[系统设置] %s", rk.Name),
			Platform: "windows",
		})
	}
	_ = tmpDir
	return entries, nil
}
