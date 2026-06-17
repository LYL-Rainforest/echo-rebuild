package store

import "fmt"

type AppEntry struct {
	Name         string `json:"name"`
	NeedManualDL bool   `json:"need_manual_dl"`
	DownloadURL  string `json:"download_url"`
	Note         string `json:"note"`
	PackagePath  string `json:"package_path"`
	IsArchive    bool   `json:"is_archive"`
	ConfigPath   string `json:"config_path"`
	Platform     string `json:"platform"`
	ScriptPath   string `json:"script_path"`
}

func (e AppEntry) Validate() error {
	if e.Name == "" {
		return fmt.Errorf("name is required")
	}
	switch e.Platform {
	case "windows", "linux", "darwin", "freebsd", "":
		return nil
	default:
		return fmt.Errorf("invalid platform: %s", e.Platform)
	}
}

type EntryCategory string

const (
	CatSoftware   EntryCategory = "software"
	CatSystem     EntryCategory = "system"
	CatDriver     EntryCategory = "driver"
)
