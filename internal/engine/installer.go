package engine

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"echo-rebuild/internal/store"
)

type Installer struct {
	DownloadDir string
}

func NewInstaller(downloadDir string) *Installer {
	return &Installer{DownloadDir: downloadDir}
}

func (inst *Installer) DownloadAndRun(ctx context.Context, entry store.AppEntry) error {
	localPath, err := inst.downloadFile(ctx, entry.DownloadURL)
	if err != nil {
		return fmt.Errorf("download %s: %w", entry.Name, err)
	}

	if err := inst.execInstaller(ctx, localPath); err != nil {
		return fmt.Errorf("exec %s: %w", entry.Name, err)
	}

	if entry.ScriptPath != "" {
		script := entry.ScriptPath
		if !filepath.IsAbs(script) {
			script = filepath.Join(inst.DownloadDir, script)
		}
		if err := inst.execScript(ctx, script); err != nil {
			return fmt.Errorf("script %s: %w", entry.Name, err)
		}
	}

	return nil
}

func (inst *Installer) CopyPortable(ctx context.Context, entry store.AppEntry, restoreBaseDir string) error {
	src := filepath.Join(restoreBaseDir, entry.PackagePath)
	dst, err := portableTargetDir(entry.Name)
	if err != nil {
		return fmt.Errorf("target dir: %w", err)
	}

	if err := copyDir(ctx, src, dst); err != nil {
		return fmt.Errorf("copy to %s: %w", dst, err)
	}

	if err := createShortcut(entry.Name, dst); err != nil {
		return fmt.Errorf("shortcut: %w", err)
	}

	return nil
}

func (inst *Installer) OpenArchive(_ store.AppEntry, restoreBaseDir string) error {
	dir := restoreBaseDir
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("dir not found: %s", dir)
	}
	return openInExplorer(dir)
}

func (inst *Installer) OpenURL(entry store.AppEntry) error {
	if entry.DownloadURL == "" {
		return fmt.Errorf("no URL")
	}
	return openBrowser(entry.DownloadURL)
}

// AutoSearchURL searches for official download URL by software name.
func (inst *Installer) AutoSearchURL(ctx context.Context, name string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	query := url.QueryEscape(name + " official download")
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", query)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return parseSearchResults(resp.Body)
}

func (inst *Installer) downloadFile(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	if err := os.MkdirAll(inst.DownloadDir, 0755); err != nil {
		return "", err
	}

	ext := ""
	if parts := strings.Split(url, "."); len(parts) > 1 {
		ext = "." + strings.Split(parts[len(parts)-1], "?")[0]
	}

	tmpFile, err := os.CreateTemp(inst.DownloadDir, "download-*"+ext)
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	written, err := io.Copy(tmpFile, resp.Body)
	if err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}
	_ = written

	return tmpFile.Name(), nil
}

func (inst *Installer) execInstaller(ctx context.Context, path string) error {
	ext := strings.ToLower(filepath.Ext(path))
	var cmd *exec.Cmd

	switch ext {
	case ".exe":
		cmd = exec.CommandContext(ctx, path, "/S")
	case ".msi":
		cmd = exec.CommandContext(ctx, "msiexec", "/i", path, "/quiet", "/norestart")
	case ".deb":
		cmd = exec.CommandContext(ctx, "dpkg", "-i", path)
	case ".rpm":
		cmd = exec.CommandContext(ctx, "rpm", "-i", path)
	case ".pkg":
		cmd = exec.CommandContext(ctx, "installer", "-pkg", path, "-target", "/")
	default:
		cmd = exec.CommandContext(ctx, path)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (inst *Installer) execScript(ctx context.Context, path string) error {
	ext := strings.ToLower(filepath.Ext(path))
	var cmd *exec.Cmd

	switch ext {
	case ".ps1":
		cmd = exec.CommandContext(ctx, "powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", path)
	case ".bat", ".cmd":
		cmd = exec.CommandContext(ctx, "cmd", "/c", path)
	case ".sh":
		cmd = exec.CommandContext(ctx, "sh", path)
	default:
		cmd = exec.CommandContext(ctx, path)
	}

	return cmd.Run()
}

func portableTargetDir(name string) (string, error) {
	var base string
	switch runtime.GOOS {
	case "windows":
		base = os.Getenv("LocalAppData")
		if base == "" {
			base = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
		}
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, "Applications")
	default:
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, sanitizeName(name)), nil
}

func sanitizeName(name string) string {
	r := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "_", "*", "_",
		"?", "_", "\"", "_", "<", "_", ">", "_", "|", "_",
	)
	return r.Replace(name)
}

func copyDir(ctx context.Context, src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(ctx, srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	r, err := os.Open(src)
	if err != nil {
		return err
	}
	defer r.Close()

	w, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer w.Close()

	_, err = io.Copy(w, r)
	return err
}

func createShortcut(name, target string) error {
	switch runtime.GOOS {
	case "windows":
		return createWindowsShortcut(name, target)
	case "darwin":
		return createMacShortcut(name, target)
	default:
		return createLinuxDesktopEntry(name, target)
	}
}

func createWindowsShortcut(name, target string) error {
	ps := fmt.Sprintf(`
$ws = New-Object -ComObject WScript.Shell
$s = $ws.CreateShortcut([Environment]::GetFolderPath('Desktop') + '\%s.lnk')
$s.TargetPath = '%s'
$s.Save()
`, sanitizeForPS1(name), sanitizeForPS1(target))
	return exec.Command("powershell", "-NoProfile", "-Command", ps).Run()
}

func sanitizeForPS1(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func createMacShortcut(name, target string) error {
	home, _ := os.UserHomeDir()
	app := filepath.Join(home, "Desktop", name+".app")
	contents := filepath.Join(app, "Contents")
	os.MkdirAll(contents, 0755)
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleExecutable</key>
	<string>%s</string>
	<key>CFBundleName</key>
	<string>%s</string>
</dict>
</plist>`, target, name)
	return os.WriteFile(filepath.Join(contents, "Info.plist"), []byte(plist), 0644)
}

func createLinuxDesktopEntry(name, target string) error {
	home, _ := os.UserHomeDir()
	desktop := filepath.Join(home, "Desktop", sanitizeName(name)+".desktop")
	content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=%s
Exec=%s
Terminal=false
`, name, target)
	return os.WriteFile(desktop, []byte(content), 0755)
}

func openInExplorer(path string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("explorer", "/select,", path).Start()
	case "darwin":
		return exec.Command("open", "-R", path).Start()
	default:
		return exec.Command("xdg-open", path).Start()
	}
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("cmd", "/c", "start", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

func parseSearchResults(r io.Reader) ([]string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	body := string(data)

	var urls []string
	seen := map[string]bool{}
	// crude HTML link extraction
	for i := 0; i < len(body)-8; i++ {
		if body[i:i+9] == `href="htt` {
			j := i + 9
			end := strings.IndexByte(body[j:], '"')
			if end < 0 {
				continue
			}
			u := body[j : j+end]
			// deduplicate, skip ads
			if !seen[u] && !strings.Contains(u, "duckduckgo") && !strings.Contains(u, "googleadservices") {
				seen[u] = true
				urls = append(urls, u)
			}
		}
	}
	if len(urls) > 5 {
		urls = urls[:5]
	}
	return urls, nil
}
