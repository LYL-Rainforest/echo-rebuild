package tbi

import (
	"archive/tar"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
)

type ImageType int

const (
	ImageRaw ImageType = iota
	ImageTBI
	ImageTarZstd
)

func (t ImageType) String() string {
	switch t {
	case ImageRaw:
		return "raw"
	case ImageTBI:
		return "tbi"
	case ImageTarZstd:
		return "tar.zstd"
	default:
		return "unknown"
	}
}

type CaptureOptions struct {
	Description  string
	Password     string
	Compression  int
	SplitVolumes bool
	Differential bool
}

type RestoreOptions struct {
	TargetDevice   string
	RepairBoot     bool
	BootRepairMode string
	ManualESPPath  string
}

type ImageInfo struct {
	Type        ImageType `json:"type"`
	Description string    `json:"description"`
	SizeBytes   int64     `json:"size_bytes"`
	CreatedAt   time.Time `json:"created_at"`
	IsSplit bool `json:"is_split"`
}

type DeviceInfo struct {
	Path       string          `json:"path"`
	SizeBytes  int64           `json:"size_bytes"`
	Model      string          `json:"model"`
	Partitions []PartitionInfo `json:"partitions"`
}

type PartitionInfo struct {
	Path       string `json:"path"`
	SizeBytes  int64  `json:"size_bytes"`
	Label      string `json:"label"`
	MountPoint string `json:"mount_point"`
}

type ImageManager struct{}

func NewImageManager() *ImageManager {
	return &ImageManager{}
}

func (m *ImageManager) Capture(ctx context.Context, sourceDevice, outputPath string, imgType ImageType, opts CaptureOptions) error {
	switch imgType {
	case ImageRaw:
		return m.captureRaw(ctx, sourceDevice, outputPath, opts)
	case ImageTarZstd:
		return m.captureTarZstd(ctx, sourceDevice, outputPath, opts)
	case ImageTBI:
		return fmt.Errorf("TBI format requires external tool, not yet bundled")
	default:
		return fmt.Errorf("unsupported image type: %s", imgType)
	}
}

func (m *ImageManager) Restore(ctx context.Context, imagePath, targetDevice string, opts RestoreOptions) error {
	imgType, err := detectImageType(imagePath)
	if err != nil {
		return err
	}
	switch imgType {
	case ImageRaw:
		return m.restoreRaw(ctx, imagePath, targetDevice, opts)
	case ImageTarZstd:
		return m.restoreTarZstd(ctx, imagePath, targetDevice, opts)
	case ImageTBI:
		return fmt.Errorf("TBI format requires external tool, not yet bundled")
	default:
		return fmt.Errorf("unsupported image type: %s", imgType)
	}
}

func (m *ImageManager) GetImageInfo(ctx context.Context, imagePath string) (*ImageInfo, error) {
	f, err := os.Open(imagePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}

	imgType, err := detectImageType(imagePath)
	if err != nil {
		return nil, err
	}

	info := &ImageInfo{
		Type:      imgType,
		SizeBytes: stat.Size(),
		CreatedAt: stat.ModTime(),
	}

	if imgType == ImageTarZstd {
		zr, err := zstd.NewReader(f)
		if err != nil {
			return info, nil
		}
		defer zr.Close()

		tr := tar.NewReader(zr)
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				break
			}
			if strings.HasSuffix(hdr.Name, "/tbi-meta.json") {
				var meta ImageInfo
				raw := make([]byte, hdr.Size)
				io.ReadFull(tr, raw)
				json.Unmarshal(raw, &meta)
				if meta.Description != "" {
					info.Description = meta.Description
				}
				break
			}
		}
	}

	return info, nil
}

func (m *ImageManager) ListDevices(ctx context.Context) ([]DeviceInfo, error) {
	switch runtime.GOOS {
	case "windows":
		return listDevicesWindows(ctx)
	case "linux":
		return listDevicesLinux(ctx)
	case "darwin":
		return listDevicesDarwin(ctx)
	default:
		return nil, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func detectImageType(path string) (ImageType, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".img", ".raw":
		return ImageRaw, nil
	case ".tbi":
		return ImageTBI, nil
	case ".zst", ".zstd":
		return ImageTarZstd, nil
	default:
		if strings.Contains(filepath.Base(path), "tar") && (strings.HasSuffix(ext, ".zst") || strings.HasSuffix(ext, ".zstd")) {
			return ImageTarZstd, nil
		}
		return ImageRaw, nil
	}
}

func (m *ImageManager) captureRaw(ctx context.Context, source, output string, opts CaptureOptions) error {
	if runtime.GOOS == "windows" {
		return m.captureRawWindows(ctx, source, output)
	}
	return m.captureRawUnix(ctx, source, output)
}

func (m *ImageManager) captureRawWindows(ctx context.Context, source, output string) error {
	ps := fmt.Sprintf(`
$src = "%s"
$dst = "%s"
$buf = New-Object byte[] 1048576
$fs = [System.IO.File]::OpenRead($src)
$fd = [System.IO.File]::OpenWrite($dst)
try {
	while ($true) {
		$read = $fs.Read($buf, 0, $buf.Length)
		if ($read -eq 0) { break }
		$fd.Write($buf, 0, $read)
	}
} finally {
	$fs.Close()
	$fd.Close()
}
`, source, output)
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", ps)
	outputBytes, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("raw capture: %s: %s", err, string(outputBytes))
	}
	return nil
}

func (m *ImageManager) captureRawUnix(ctx context.Context, source, output string) error {
	cmd := exec.CommandContext(ctx, "dd", "if="+source, "of="+output, "bs=1M", "status=progress")
	outputBytes, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("raw capture: %s: %s", err, string(outputBytes))
	}
	return nil
}

func (m *ImageManager) captureTarZstd(ctx context.Context, source, output string, opts CaptureOptions) error {
	// source is a mount point / directory to back up
	compressionLevel := opts.Compression
	if compressionLevel <= 0 {
		compressionLevel = 3
	}

	f, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	defer f.Close()

	zw, err := zstd.NewWriter(f, zstd.WithEncoderLevel(zstd.EncoderLevel(compressionLevel)))
	if err != nil {
		return fmt.Errorf("zstd writer: %w", err)
	}
	defer zw.Close()

	tw := tar.NewWriter(zw)
	defer tw.Close()

	// Write metadata
	if opts.Description != "" {
		meta := ImageInfo{
			Type:        ImageTarZstd,
			Description: opts.Description,
			CreatedAt:   time.Now(),
		}
		metaBytes, _ := json.Marshal(meta)
		hdr := &tar.Header{
			Name:     ".tbi-meta.json",
			Size:     int64(len(metaBytes)),
			Mode:     0644,
			ModTime:  time.Now(),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("meta header: %w", err)
		}
		if _, err := tw.Write(metaBytes); err != nil {
			return fmt.Errorf("meta write: %w", err)
		}
	}

	return filepath.Walk(source, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		relPath, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}
		relPath = filepath.ToSlash(relPath)

		hdr, err := tar.FileInfoHeader(fi, "")
		if err != nil {
			return fmt.Errorf("header %s: %w", relPath, err)
		}
		hdr.Name = relPath
		if fi.IsDir() {
			hdr.Name += "/"
		}

		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("write header %s: %w", relPath, err)
		}

		if !fi.Mode().IsRegular() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open %s: %w", relPath, err)
		}
		defer f.Close()

		written, err := io.Copy(tw, f)
		if err != nil {
			return fmt.Errorf("copy %s: %w", relPath, err)
		}
		_ = written
		return nil
	})
}

func (m *ImageManager) restoreRaw(ctx context.Context, image, target string, opts RestoreOptions) error {
	if runtime.GOOS == "windows" {
		return m.restoreRawWindows(ctx, image, target)
	}
	return m.restoreRawUnix(ctx, image, target)
}

func (m *ImageManager) restoreRawWindows(ctx context.Context, image, target string) error {
	ps := fmt.Sprintf(`
$src = "%s"
$dst = "%s"
$buf = New-Object byte[] 1048576
$fs = [System.IO.File]::OpenRead($src)
$fd = [System.IO.File]::OpenWrite($dst)
try {
	while ($true) {
		$read = $fs.Read($buf, 0, $buf.Length)
		if ($read -eq 0) { break }
		$fd.Write($buf, 0, $read)
	}
} finally {
	$fs.Close()
	$fd.Close()
}
`, image, target)
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", ps)
	outputBytes, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("raw restore: %s: %s", err, string(outputBytes))
	}
	return nil
}

func (m *ImageManager) restoreRawUnix(ctx context.Context, image, target string) error {
	cmd := exec.CommandContext(ctx, "dd", "if="+image, "of="+target, "bs=1M", "status=progress")
	outputBytes, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("raw restore: %s: %s", err, string(outputBytes))
	}
	return nil
}

func (m *ImageManager) restoreTarZstd(ctx context.Context, image, target string, opts RestoreOptions) error {
	f, err := os.Open(image)
	if err != nil {
		return fmt.Errorf("open image: %w", err)
	}
	defer f.Close()

	zr, err := zstd.NewReader(f)
	if err != nil {
		return fmt.Errorf("zstd reader: %w", err)
	}
	defer zr.Close()

	tr := tar.NewReader(zr)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read: %w", err)
		}

		if hdr.Name == ".tbi-meta.json" {
			continue
		}

		targetPath := filepath.Join(target, hdr.Name)

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(hdr.Mode)); err != nil {
				return fmt.Errorf("mkdir %s: %w", targetPath, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("mkdir parent %s: %w", targetPath, err)
			}
			of, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return fmt.Errorf("create %s: %w", targetPath, err)
			}
			_, err = io.Copy(of, tr)
			of.Close()
			if err != nil {
				return fmt.Errorf("write %s: %w", targetPath, err)
			}
		case tar.TypeSymlink:
			if err := os.Symlink(hdr.Linkname, targetPath); err != nil {
				return fmt.Errorf("symlink %s: %w", targetPath, err)
			}
		case tar.TypeLink:
			if err := os.Link(hdr.Linkname, targetPath); err != nil {
				return fmt.Errorf("link %s: %w", targetPath, err)
			}
		}
	}

	if opts.RepairBoot {
		return repairBoot(target, opts)
	}
	return nil
}

func listDevicesWindows(ctx context.Context) ([]DeviceInfo, error) {
	ps := `Get-Partition | Where-Object DriveLetter | ForEach-Object {
		$disk = Get-Disk -Number $_.DiskNumber
		$vol = Get-Volume -DriveLetter $_.DriveLetter
		[PSCustomObject]@{
			DiskNumber   = $_.DiskNumber
			DriveLetter  = $_.DriveLetter
			Size         = $_.Size
			Type         = $_.Type
			DiskModel    = $disk.FriendlyName
			FileSystem   = $vol.FileSystem
			SizeRemaining = $vol.SizeRemaining
		}
	} | ConvertTo-Json -Compress`
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", ps)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list partitions: %w", err)
	}

	var raw []struct {
		DiskNumber    int    `json:"DiskNumber"`
		DriveLetter   string `json:"DriveLetter"`
		Size          int64  `json:"Size"`
		Type          string `json:"Type"`
		DiskModel     string `json:"DiskModel"`
		FileSystem    string `json:"FileSystem"`
		SizeRemaining int64  `json:"SizeRemaining"`
	}
	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, fmt.Errorf("parse partitions: %w", err)
	}

	var devices []DeviceInfo
	for _, d := range raw {
		if d.DriveLetter == "" {
			continue
		}
		label := fmt.Sprintf("Disk %d - %s | %s:  %s  (%d GB / %d GB 可用)",
			d.DiskNumber, d.DiskModel, d.DriveLetter+`:`, d.FileSystem,
			d.Size>>30, d.SizeRemaining>>30)
		devices = append(devices, DeviceInfo{
			Path:      d.DriveLetter + ":\\",
			SizeBytes: d.Size,
			Model:     label,
		})
	}
	return devices, nil
}

func listDevicesLinux(ctx context.Context) ([]DeviceInfo, error) {
	cmd := exec.CommandContext(ctx, "lsblk", "-J", "-o", "NAME,SIZE,TYPE,MOUNTPOINT,FSTYPE,PKNAME")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("lsblk: %w", err)
	}

	var parsed struct {
		BlockDevices []struct {
			Name       string `json:"name"`
			Size       string `json:"size"`
			Type       string `json:"type"`
			FSType     string `json:"fstype"`
			MountPoint string `json:"mountpoint"`
			PkName     string `json:"pkname"`
		} `json:"blockdevices"`
	}
	if err := json.Unmarshal(output, &parsed); err != nil {
		return nil, fmt.Errorf("parse lsblk: %w", err)
	}

	var devices []DeviceInfo
	for _, bd := range parsed.BlockDevices {
		if bd.Type != "part" && bd.Type != "crypt" {
			continue
		}
		size := parseSize(bd.Size)
		parent := bd.PkName
		if parent == "" {
			parent = "-"
		}
		label := fmt.Sprintf("/dev/%s (%s)  [%s]  <- %s", bd.Name, bd.FSType, bd.Size, parent)
		if bd.MountPoint != "" {
			label += fmt.Sprintf(" 挂载: %s", bd.MountPoint)
		}
		devices = append(devices, DeviceInfo{
			Path:      "/dev/" + bd.Name,
			SizeBytes: size,
			Model:     label,
		})
	}
	return devices, nil
}

func listDevicesDarwin(ctx context.Context) ([]DeviceInfo, error) {
	cmd := exec.CommandContext(ctx, "diskutil", "list")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("diskutil: %w", err)
	}

	// Parse diskutil output to show partition tree
	lines := strings.Split(string(out), "\n")
	var devices []DeviceInfo
	currentDisk := ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "/dev/disk") {
			if !strings.Contains(trimmed, "s") {
				currentDisk = strings.Fields(trimmed)[0]
			} else {
				parts := strings.Fields(trimmed)
				if len(parts) >= 2 {
					size := parseSize(strings.TrimSuffix(parts[len(parts)-1], "*"))
					label := fmt.Sprintf("%s %s <- %s", parts[0], strings.Join(parts[1:], " "), currentDisk)
					devices = append(devices, DeviceInfo{
						Path:      parts[0],
						SizeBytes: size,
						Model:     label,
					})
				}
			}
		}
	}
	return devices, nil
}

func parseSize(s string) int64 {
	s = strings.ToUpper(strings.TrimSpace(s))
	var multiplier int64 = 1
	switch {
	case strings.HasSuffix(s, "TB"):
		multiplier = 1 << 40
		s = strings.TrimSuffix(s, "TB")
	case strings.HasSuffix(s, "GB"):
		multiplier = 1 << 30
		s = strings.TrimSuffix(s, "GB")
	case strings.HasSuffix(s, "MB"):
		multiplier = 1 << 20
		s = strings.TrimSuffix(s, "MB")
	case strings.HasSuffix(s, "KB"):
		multiplier = 1 << 10
		s = strings.TrimSuffix(s, "KB")
	default:
		return 0
	}
	var val float64
	fmt.Sscanf(s, "%f", &val)
	return int64(val * float64(multiplier))
}

func repairBoot(target string, opts RestoreOptions) error {
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("bcdboot", target+"\\Windows", "/s", opts.ManualESPPath)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("bcdboot: %s: %s", err, string(output))
		}
	case "linux":
		cmd := exec.Command("grub-install", "--target=x86_64-efi", "--efi-directory="+opts.ManualESPPath, "--bootloader-id=GRUB")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("grub-install: %s: %s", err, string(output))
		}
	case "darwin":
		cmd := exec.Command("bless", "--mount", target, "--setBoot")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("bless: %s: %s", err, string(output))
		}
	}
	return nil
}
