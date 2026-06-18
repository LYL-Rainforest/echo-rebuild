# EchoRebuild — 完整项目规格说明书
> 智能体按此文件可直接写出完整项目代码。每个章节说明「写什么」「写到什么程度」「与其他章节如何关联」。

---

## 1. 项目定位

单可执行文件，内嵌 SQLite，四大核心功能：

| # | 功能 | 说明 |
|---|------|------|
| 1 | 创建系统配置 | 扫描本机软件/驱动/系统设置/注册表，生成 `.db` 配置数据库文件 |
| 2 | 还原系统配置 | 读取 `.db`，选择性跨平台恢复软件配置，并行下载安装，失败降级手动 |
| 3 | 创建系统镜像 | 对指定磁盘/分区创建物理镜像，支持 raw / tbi / tar.zstd 三种格式 |
| 4 | 还原系统镜像 | 选择镜像文件 → 选择目标盘 → 修复引导 → 写入

## 2. 模块依赖关系图（数据流）

```
cmd/echo/         (TUI 层)
  menu.go
    ├── pages/image_backup/  (7 步向导)
    ├── pages/image_restore/ (5 步向导)
    ├── pages/progress.go    (进度展示)
    └── pages/result.go      (结果汇总)
            │
            ▼
workflow.go ────┬─── scanner.go ──── scanner_windows.go / scanner_unix.go
                │                       │
                │                       ▼
                │                   store/models.go (AppEntry)
                │
                ├─── installer.go ──── pool.go (并发引擎)
                │       │
                │       ▼
                │   store/db.go (读写 SQLite)
                │
                └─── tbi/manager.go (raw / tbi / tar.zstd 镜像引擎)
```

**核心数据实体**：`AppEntry`（定义在 `store/models.go`）是所有模块的通信载体。
**TUI 交互**：用户通过 TUI 菜单选择 → `workflow.go` 执行 → 结果回传给 TUI 展示。

## 3. 编码约定（所有 .go 文件必须遵守）

| 规则 | 说明 |
|------|------|
| **错误处理** | 所有可能失败的函数返回 `error`，使用 `fmt.Errorf("context: %w", err)` 包裹 |
| **日志** | 使用标准库 `log` 包，不要第三方日志库 |
| **命名** | 导出类型/函数用驼峰，包名全小写无下划线 |
| **零值** | 结构体零值必须可用，避免 nil 指针 |
| **Context** | 所有可能阻塞的函数首参数为 `context.Context` |
| **并发** | 用 `sync.WaitGroup` + channel，不要裸 `go` 不管理 |
| **注释** | 导出类型/函数必须有 doc comment，行内不要废话注释 |
| **跨平台** | 平台差异通过 Build Tags 隔离，`scanner.go` 提供统一接口 |
| **导入顺序** | std → 第三方(bubbletea等) → 内部，每组空行分隔 |
| **文件头** | 不写版权声明，直接 `package xxx` |
| **TUI** | 使用 `github.com/charmbracelet/bubbletea` 构建终端交互界面，不用 GUI |

## 4. 构建规范

- 模块名：`echo-rebuild`
- go.mod 只保留 `module echo-rebuild` + `go 1.26`，不要多余指示
- 编译产物 → `build/` 目录
- 目标平台：`windows/amd64` `linux/amd64` `darwin/amd64`
- CGO_ENABLED=1（依赖 `mattn/go-sqlite3`）
- Build Tags：`scanner_windows.go` 用 `//go:build windows`；`scanner_unix.go` 用 `//go:build linux || darwin || freebsd`
- 第三方依赖：
  - `github.com/mattn/go-sqlite3` — SQLite 驱动
  - `github.com/charmbracelet/bubbletea` — TUI 框架
  - `github.com/charmbracelet/lipgloss` — TUI 样式
  - `github.com/charmbracelet/bubbles` — TUI 通用组件（table, textinput 等）

- `build.sh` 脚本循环三个平台执行 `CGO_ENABLED=1 GOOS=xxx GOARCH=amd64 go build -o build/echo-rebuild-$GOOS-$GOARCH ./cmd/echo/`

## 5. 数据库核心模型 — AppEntry

### 5.1 结构体定义（`store/models.go`）

```go
type AppEntry struct {
    Name         string `json:"name"`          // 唯一标识，PRIMARY KEY
    NeedManualDL bool   `json:"need_manual_dl"` // true=不自动下载
    DownloadURL  string `json:"download_url"`   // 自动拉取 URL
    Note         string `json:"note"`           // 用户自定义备忘
    PackagePath  string `json:"package_path"`   // 相对路径（绿色版目录/压缩包）
    IsArchive    bool   `json:"is_archive"`     // true=压缩包需手动解压；false=免安装直接复制
    ConfigPath   string `json:"config_path"`    // 配置文件/数据存放目录
    Platform     string `json:"platform"`       // windows/linux/darwin/freebsd/""(全平台)
    ScriptPath   string `json:"script_path"`    // 配置脚本路径
}
```

### 5.2 字段约束

| 字段 | 约束 |
|------|------|
| Name | 非空，唯一，长度 1-255 |
| Platform | 取值只能是 `windows` `linux` `darwin` `freebsd` 或 `""` |
| NeedManualDL | true 时 DownloadURL 必填 |
| DownloadURL | 空字符串表示无下载来源 |
| Note | 自由文本，无限制 |
| PackagePath | 可为空（空=走URL下载）；非空=相对路径（目录或压缩包） |
| IsArchive | true=压缩包，还原时只打开路径让用户手动处理；false=免安装目录，还原时复制目录+创建桌面快捷方式 |
| ConfigPath | 可为空 |
| ScriptPath | 可为空 |

### 5.3 SQL 建表语句

```sql
CREATE TABLE IF NOT EXISTS app_entries (
    name           TEXT PRIMARY KEY,
    need_manual_dl INTEGER NOT NULL DEFAULT 0,
    download_url   TEXT NOT NULL DEFAULT '',
    note           TEXT NOT NULL DEFAULT '',
    package_path   TEXT NOT NULL DEFAULT '',
    is_archive     INTEGER NOT NULL DEFAULT 0,
    config_path    TEXT NOT NULL DEFAULT '',
    platform       TEXT NOT NULL DEFAULT '',
    script_path    TEXT NOT NULL DEFAULT ''
);
```

### 5.4 关联说明

- `scanner_windows.go` / `scanner_unix.go` **填充** AppEntry
- `store/db.go` **持久化/读取** AppEntry
- `workflow.go` **编排** AppEntry 的流动（scan→save→load→install）
- `installer.go` **消费** AppEntry 的 DownloadURL / PackagePath / ScriptPath

## 6. 职责划分与工作流

> 核心原则：**TUI 负责交互选择，workflow 负责执行，互不越界。**

```
TUI (pages/)                   workflow.go                 下层模块
─────                         ──────────                  ────────
扫描 + 显示进度               scanner.Scan()
树选 + 手动填路径             
确认 → BackupConfig(entries)  ──→  SaveEntries(db, entries)  store/db.go
                               ←──  ok
显示结果

选择 .db + 树选恢复项          
确认 → RestoreConfig(entries)  ──→ 遍历 entries: 
                  分支 1 (IsArchive)  → Installer.OpenArchive()
                  分支 2 (PackagePath) → Installer.CopyPortable()
                  分支 3 (DownloadURL) → Installer.DownloadAndRun()
                  分支 4 (else)        → skipped
                                ←──  results[]
显示结果汇总
```

### 6.1 TUI 层交互流程

**创建系统配置**（完整步骤）：

```
① 用户选 "2. 创建系统配置"
② TUI 调用 scanner.New().Scan(ctx) → 显示进度
③ 返回 []AppEntry（仅填充 Name + Platform）
④ TUI 组织为三级分类树展示
⑤ 用户 ↑↓→← Space 勾选条目
⑥ 用户按 p 逐一设置安装来源（URL/免安装目录/压缩包）
⑦ 用户按 Enter 确认
⑧ TUI 收集所有选中且已设来源的条目 → 调用 workflow.BackupConfig(db, entries)
⑨ workflow 直接 SaveEntries → 返回
⑩ TUI 显示完成页
```

**还原系统配置**（完整步骤）：

```
① 用户选 "3. 还原系统配置"
② TUI 显示文件选择器 → 用户选 .db
③ TUI 调用 store.LoadEntries(db, platform=runtime.GOOS) → 过滤不匹配平台的条目
④ TUI 组织为三级分类树展示（默认全选）
⑤ 用户 ↑↓→← Space 微调
⑥ 用户按 Enter 确认
⑦ TUI 收集被选中的条目 → 调用 workflow.RestoreConfig(entries, restoreBaseDir)
⑧ workflow 创建 WorkerPool，按条目类型分发：
   ├─ IsArchive      → Installer.OpenArchive()
   ├─ PackagePath≠"" → Installer.CopyPortable()
   ├─ DownloadURL≠"" → Installer.DownloadAndRun()
   └─ 无来源          → skipped
⑨ 收集结果 → 返回 TUI
⑩ TUI 显示结果汇总页
```

### 6.2 workflow 函数签名

```go
// BackupConfig — 直接存库，不做扫描
func (w *Workflow) BackupConfig(ctx context.Context, entries []store.AppEntry) error

// RestoreConfig — 加载已过滤的条目，并发安装
func (w *Workflow) RestoreConfig(ctx context.Context, entries []store.AppEntry, restoreBaseDir string) *RestoreSummary

type RestoreSummary struct {
    Success     int      // 成功安装
    Manual      int      // 压缩包，已打开路径
    ManualNames []string // 需要手动操作的条目名称列表
    Fallback    int      // URL 下载失败，已打开浏览器
    FallbackNames []string
    Skipped     int      // 无来源
    SkippedNames []string
}
```

**错误处理**：
- `BackupConfig`：SaveEntries 失败 → 回滚事务，返回 error
- `RestoreConfig`：单个 Task 失败不影响其他 Task；全部执行完后返回汇总

## 7. 各 .go 文件精确规格（按实现顺序排列）

### 7.1 `internal/store/models.go` ⭐ 先写（被所有包引用）

**写什么**：
- `AppEntry` 结构体（8 字段，json tag）
- `Validate()` 方法：检查 Name 非空、Platform 合法
- `SystemSnapshot` 结构体（可选，用于 TBI 备份元数据）

**函数签名**：
```go
type AppEntry struct { ... }  // 9 个字段
func (e AppEntry) Validate() error  // 字段校验
```

**关联**：`scanner.go` 返回 `[]AppEntry`，`db.go` 读写 `[]AppEntry`，`installer.go` 消费 `AppEntry`。

---

### 7.2 `internal/store/db.go` ⭐ 第二批

**写什么**：
- SQLite 数据库初始化、建表、CRUD

**函数签名**：
```go
func InitDB(path string) (*sql.DB, error)
func SaveEntries(db *sql.DB, entries []AppEntry) error     // 事务批量 INSERT OR REPLACE
func LoadEntries(db *sql.DB, platform string) ([]AppEntry, error) // platform="" 返回全部
func DeleteEntry(db *sql.DB, name string) error
func CloseDB(db *sql.DB) error
```

**实现要求**：
- `InitDB` 调用 `sql.Open("sqlite3", path)`，执行 CREATE TABLE
- `SaveEntries` 用 `BEGIN` + `PREPARE` + 循环执行 + `COMMIT`
- `LoadEntries` 支持 WHERE platform IN ("", $1) 过滤
- `CloseDB` 调用 db.Close()
- 所有函数使用 `context.Background()` 或从参数传入

**关联**：`workflow.go` BackupConfig / RestoreConfig 均调用此包。TUI 页面在还原时也直接调用 LoadEntries。

---

### 7.3 `internal/scanner/scanner.go` ⭐ 第三批（接口定义）

**写什么**：
- `Scanner` 接口
- `New()` 工厂函数（Build Tags 决定具体实现）
- `ScanOptions` 结构体（可扩展）

**函数签名**：
```go
type Scanner interface {
    Scan(ctx context.Context, opts ScanOptions) ([]store.AppEntry, error)
}

type ScanOptions struct {
    Platform string // 限制扫描目标平台
}

func New() Scanner  // 由 scanner_windows.go / scanner_unix.go 各自实现
```

**关联**：`workflow.go` RestoreConfig 调用此包的 LoadEntries；TUI 页面调用 scanner.New().Scan()。

---

### 7.4 `internal/scanner/scanner_windows.go` ⭐ + `scanner_unix.go`

**Build Tags**：
- `scanner_windows.go`：`//go:build windows`
- `scanner_unix.go`：`//go:build linux || darwin || freebsd`

**写什么**（以 Windows 为例）：
```go
type windowsScanner struct{}

func init() { newScanner = func() Scanner { return &windowsScanner{} } }

func (s *windowsScanner) Scan(ctx context.Context, _ ScanOptions) ([]store.AppEntry, error)
```

**Windows 扫描逻辑**：
1. 打开注册表 `HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`
2. 枚举子项，读取 DisplayName、InstallLocation、Publisher
3. 对每个有效的 DisplayName 构造 AppEntry：
   - Name = `"[软件] " + DisplayName`（前缀 `[软件]` 使分类树能区分）
   - Platform = "windows"
   - 其他字段留空（用户可在备份树中手动设置）
4. 系统设置导出：执行 `reg export HKLM\SYSTEM\CurrentControlSet\Control\Session Manager\Environment` 等，
   生成 .reg 临时文件，构造 AppEntry：
   - Name = `"[系统设置] " + 设置项名称`（如 `[系统设置] 当前用户环境变量`）
   - Platform = "windows"
   - 其他字段留空

**Unix 扫描逻辑**：
1. Linux：解析 `/var/lib/dpkg/status` 或执行 `dpkg-query -W -f '${Package}\t${Version}\n'`
2. macOS：执行 `brew list --cask --versions`
3. FreeBSD：执行 `pkg info`
4. 每个条目：
   - Name = package name
   - Platform = runtime.GOOS
   - PackagePath/ConfigPath 留空

**关联**：`scanner.go` 调用实现，`workflow.go` 消费返回的 `[]AppEntry`。

---

### 7.5 `internal/engine/pool.go` ⭐ 第四批

**写什么**：
- 通用 WorkerPool，独立于具体任务类型

**函数签名**：
```go
type Job struct {
    Data any
}

type Result struct {
    Job  Job
    Err  error
    Info string  // "success" / "manual" / "fallback" / "skipped"
    Name string  // entry name for tracking
}

type WorkerPool struct {
    workers int
    wg      sync.WaitGroup
    cancel  context.CancelFunc
}

func NewPool(n int) *WorkerPool

// Start 返回一个 send-only jobs channel 和一个 receive-only results channel
func (p *WorkerPool) Start(ctx context.Context, handler func(context.Context, any) Result) (chan<- Job, <-chan Result)

// Wait 阻塞直到所有 worker 退出
func (p *WorkerPool) Wait()
```

**实现要求**：
- 使用泛型 `[T any]` 保持通用
- worker 数量默认 `runtime.NumCPU()`
- 每个 worker 从 jobs channel 读取 job，执行函数（通过闭包注入），发送结果
- 关闭 jobs channel 后 workers 优雅退出
- 支持 context 取消

**关联**：`workflow.go` Restore 用此包并行分发安装任务。

---

### 7.6 `internal/engine/installer.go` ⭐ 第五批

**写什么**：
- 安装引擎，按软件类型走不同处理路径
- URL 搜索：根据软件名称自动搜索官方下载地址

**函数签名**：
```go
type Installer struct {
    DownloadDir string  // URL 安装包缓存目录
}

func NewInstaller(downloadDir string) *Installer

// DownloadAndRun — 有 DownloadURL 的安装包：下载 → 执行 → 脚本
func (inst *Installer) DownloadAndRun(ctx context.Context, entry store.AppEntry) error

// CopyPortable — 免安装目录：复制文件夹 → 创建桌面快捷方式
func (inst *Installer) CopyPortable(ctx context.Context, entry store.AppEntry, restoreBaseDir string) error

// OpenArchive — 压缩包：在文件管理器中打开所在目录
func (inst *Installer) OpenArchive(entry store.AppEntry, restoreBaseDir string) error

// OpenURL — 打开浏览器让用户手动下载
func (inst *Installer) OpenURL(entry store.AppEntry) error

// AutoSearchURL — 根据软件名称搜索官方下载地址，返回 URL 列表
func (inst *Installer) AutoSearchURL(ctx context.Context, name string) ([]string, error)
```

**实现要求**：
- `DownloadAndRun`：
  1. HTTP GET 下载到 DownloadDir，支持重定向
  2. 执行安装包（exec.Command）：Win 用 `.exe` / `.msi`，Unix 用 `.deb` / `.rpm` / `.pkg`
  3. 若 ScriptPath 非空，执行配置脚本
  4. 任意步骤失败 → 返回 error（上层决定是否 OpenURL）
- `CopyPortable`：
  1. 拼接 `restoreBaseDir / PackagePath`
  2. 复制整个目录到目标位置（默认 `~/.local/share/{Name}` 或 `%LocalAppData%\{Name}`）
  3. 创建桌面快捷方式：Win 用 COM 创建 .lnk；Linux 创建 .desktop；macOS 用 `osascript`
- `OpenArchive`：在文件管理器打开压缩包所在目录（Win→`explorer /select,`；Linux→`xdg-open`；macOS→`open -R`）
- `OpenURL`：
  - Windows：`cmd /c start {url}`
  - Darwin：`open {url}`
  - Linux：`xdg-open {url}`
- `AutoSearchURL`：
  - 使用搜索引擎 API（如 DuckDuckGo 或 Google Custom Search）搜索 `"{软件名} 官方下载"` 或 `"{software name} official download"`
  - 返回结果前 5 个 URL
  - 超时 10 秒
  - 失败返回空列表 + error（TUI 层降级为手动输入）

---

### 7.7 `internal/tbi/manager.go` ⭐ 系统镜像引擎

**写什么**：
- 支持三种镜像格式的系统备份/还原引擎

**镜像类型**：
| 类型 | 说明 | 工具依赖 | 平台 |
|------|------|----------|------|
| TBI | 磁盘级，True Image 格式 | 内嵌 tbi 二进制 | 全平台 |
| IMG/raw | 原始扇区镜像 | 内置实现（智能扇区复制） | 全平台 |
| tar.zstd | 文件级，tar + Zstandard 压缩 | 内置（archive/tar + github.com/klauspost/compress/zstd） | 全平台 |

**函数签名**：
```go
type ImageType int
const (
    ImageRaw ImageType = iota     // 智能扇区复制，全平台
    ImageTBI                       // 块级 True Image，全平台
    ImageTarZstd                   // 文件级 tar+Zstd，全平台
)

type CaptureOptions struct {
    Description  string
    Password     string
    Compression  int      // tar.zstd: 0-19 (0=不压缩, 3=默认); TBI: 映射; Raw: 无效
    SplitVolumes bool     // 是否分卷(>4GB自动分割)
    Differential bool     // 是否差分（基于上次全量）
}

type RestoreOptions struct {
    TargetDevice string   // 目标盘符或设备路径
    RepairBoot   bool     // 是否修复引导
    BootRepairMode string // "auto" / "manual" / "skip"
    ManualESPPath string  // 手动指定 EFI 分区路径
}

type ImageInfo struct {
    Type        ImageType
    Description string
    SizeBytes   int64
    CreatedAt   time.Time
    HasPassword bool
    IsSplit     bool    // 是否分卷
}

type ImageManager struct {
    // 根据镜像类型选择底层工具
}

func NewImageManager() *ImageManager

// Capture 创建系统镜像
func (m *ImageManager) Capture(ctx context.Context, sourceDevice, outputPath string, imgType ImageType, opts CaptureOptions) error

// Restore 还原系统镜像
func (m *ImageManager) Restore(ctx context.Context, imagePath, targetDevice string, opts RestoreOptions) error

// GetImageInfo 读取镜像元信息
func (m *ImageManager) GetImageInfo(ctx context.Context, imagePath string) (*ImageInfo, error)

// ListDevices 列出可用磁盘/分区
func (m *ImageManager) ListDevices(ctx context.Context) ([]DeviceInfo, error)
```

**实现要求**：
- `ListDevices`：Windows → `powershell Get-Disk | Get-Partition`；Unix → `lsblk` / `diskutil list`
- `Capture`：根据 ImageType 选择不同实现路径
  - TBI → 调用内嵌 tbi 二进制
  - Raw → 智能扇区复制：读取分区表 → 解析文件系统(MFT/FAT/ext) → 仅复制已使用的数据扇区，跳过空白/零填充扇区。输出为稀疏文件或按实际数据量压缩。Windows 用 `FSCTL_GET_NTFS_VOLUME_DATA` + `FSCTL_GET_RETRIEVAL_POINTERS`，Linux 用 `fiemap` 或 `lseek(SEEK_DATA/SEEK_HOLE)`。
  - tar.zstd → 使用 `archive/tar` 遍历文件系统，用 `github.com/klauspost/compress/zstd` 压缩流式写入
- `Restore`：同 Capture，逆操作
- 分卷：TBI 内置支持；Raw 按大小分割 .img.001/.img.002…；tar.zstd 流式可分割

- 压缩等级：tar.zstd 对应 zstd 压缩级别 0~19
- 修复引导：跨平台实现
  - Windows → `bcdboot` / `bootrec`
  - Linux → `grub-install` / `update-grub`
  - macOS → `bless` / `diskutil`

**关联**：`workflow.go` 调用此模块，TUI 页面调用 `workflow.go`。

---

### 7.8 `internal/app/workflow.go` ⭐ 第六批（组装层）

**写什么**：
- Workflow 编排器，按 TUI 传参直接调下层模块，不做交互决策
- 四大核心工作流的完整实现

**函数签名**：
```go
type Workflow struct {
    db     *sql.DB
    dbPath string
}

func NewWorkflow(dbPath string) (*Workflow, error)
func (w *Workflow) DB() *sql.DB
func (w *Workflow) Close() error

// 配置备份：TUI 已选好条目 → 直接存库
func (w *Workflow) BackupConfig(ctx context.Context, entries []store.AppEntry) error

// 配置恢复：TUI 已选好条目 → 并发安装 → 返回汇总
func (w *Workflow) RestoreConfig(ctx context.Context, entries []store.AppEntry, restoreBaseDir string) *RestoreSummary

// 系统镜像：委托 ImageManager
func (w *Workflow) CaptureImage(ctx context.Context, source, output string, imgType ImageType, opts CaptureOptions) error
func (w *Workflow) RestoreImage(ctx context.Context, image, target string, opts RestoreOptions) error
```

**BackupConfig 实现**：
```go
// 调用 store.SaveEntries(db, entries)，事务写入
// 成功返回 nil，失败回滚并返回 error
```

**RestoreConfig 实现**：
```go
pool := NewPool(runtime.NumCPU())
ctx, cancel := context.WithCancel(ctx)
defer cancel()
jobs, results := pool.Start[store.AppEntry](ctx)

go func() {
    defer close(jobs)
    for _, entry := range entries {
        jobs <- Job[store.AppEntry]{Data: entry}
    }
}()

summary := &RestoreSummary{}
for result := range results {
    entry := result.Job.Data
    switch {
    case entry.IsArchive:
        Installer.OpenArchive(entry, restoreBaseDir)
        summary.Manual++
        summary.ManualNames = append(summary.ManualNames, entry.Name)
    case entry.PackagePath != "":
        Installer.CopyPortable(ctx, entry, restoreBaseDir)
        summary.Success++
    case entry.DownloadURL != "":
        err := Installer.DownloadAndRun(ctx, entry)
        if err != nil {
            Installer.OpenURL(entry)
            summary.Fallback++
            summary.FallbackNames = append(summary.FallbackNames, entry.Name)
        } else {
            summary.Success++
        }
    default:
        summary.Skipped++
        summary.SkippedNames = append(summary.SkippedNames, entry.Name)
    }
}
pool.Wait()
return summary
```

**CaptureImage / RestoreImage 实现**：
- 直接调用 `tbi.NewImageManager().Capture()` / `.Restore()`
- 透传参数，不做额外处理

**关联**：TUI 所有页面（pages_*.go）均调用此包的函数。

---

### 7.9 `pkg/embed/assets.go` ⭐ 最后

**写什么**：
- 内嵌 TBI 二进制文件

```go
package embed

import "embed"

//go:embed tbi/*
var Assets embed.FS
```

**关联**：`tbi/manager.go` 通过 `Assets.ReadFile("tbi/xxx")` 获取内嵌二进制。

---

### 7.10 `cmd/echo/main.go` ⭐ 最后（TUI 入口）

**写什么**：
- TUI（终端交互界面）入口，启动 bubbletea 程序
- 主 Model 状态机：根据 `page` 枚举值分发到各子页面
- 不解析 CLI flags（所有参数通过菜单交互提供）

**函数签名**：
```go
type page int
const (
    pageMainMenu page = iota
    pageConfigBackup
    pageConfigRestore
    pageImageBackup
    pageImageRestore
)

type MainModel struct {
    page    page
    menu    MainMenuModel
    configB *ConfigBackupModel
    configR *ConfigRestoreModel
    imageB  *ImageBackupModel
    imageR  *ImageRestoreModel
    width   int
    height  int
}

func NewMainModel() MainModel  // 初始 page = pageMainMenu
func (m MainModel) Init() tea.Cmd
func (m MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd)
func (m MainModel) View() string
```

**实现要求**：
- `main()` 直接启动 bubbletea `program.Run()`，使用 `tea.WithAltScreen()`
- Update 中按 `m.page` 路由到对应子 Model 的 Update，并更新引用
- 子 Model 通过返回 `menuChoice` msg 通知主 Model 切换页面
- 子 Model 返回 `menuChoice(0)` = 返回主菜单
- 所有页面数据流：
  ```
  menuChoice msg → 主 Model 创建子 Model →
  子 Model 调用 workflow 函数 →
  子 Model 返回结果 msg →
  子 Model 显示完成页 →
  用户按 Enter → 返回 menuChoice(0) → 主菜单
  ```

**关联**：唯一调用 `workflow.go` 的包，也是编译入口点。所有界面元素引用第 10 节的 TUI 设计。

---

## 8. 开发协议

### 8.1 预估与超时

每个模块编码前必须先硬性预估耗时，写在 commit message 或日志中：

```
模块: internal/store/db.go
预估: 25min
实际: ?
```

**超时规则**：

| 预估 | 容忍上限 | 超时处理 |
|------|----------|----------|
| ≤10min | 2× | 立即停止，退回上一步思考 |
| 10~30min | 1.5× | 立即停止，退回上一步思考 |
| 30~60min | 1.3× | 立即停止，退回上一步思考 |
| >60min | 1.2× | 立即停止，退回上一步思考 |

> 例如：预估 25min → 37.5min 未完成 → 必须停止，不能"再给 5 分钟"

**超时后动作流程**：
1. 停止当前编码，保存已改文件
2. 输出当前卡点（哪一行/哪个逻辑过不去）
3. 用 VSCode API 检查相关文件的实际内容
4. 分析根本原因：是设计遗漏？类型不对？API 理解错？
5. 修改设计或换方案后重新预估，再继续

### 8.2 报错处理策略

编码中遇任何非预期报错时，按以下优先级解决：

```
① 读报错全文（不要只看最后一行）
② 去对应文件 grep/read 定位出错行
③ 分析原因类型：
   ├─ 类型/签名不匹配  → 查调用链，修签名或调用处
   ├─ 包/模块未导入    → go mod tidy + 检查 import
   ├─ 逻辑条件遗漏     → 补分支/guard clause
   ├─ 外部依赖异常     → 检查 API 返回值/文件是否存在
   └─ 不理解的行为     → 写最小可复现片段单独测试
④ 修复后立即 go build（或 go vet）验证
⑤ 不通过则回到 ③，循环最多 3 轮
⑥ 3 轮未解决 → 超时处理
```

**不允许的行为**：
- 注释掉报错行先跳过
- 随便改个类型让编译通过但不理解原因
- 连续 3 次相同报错不换思路

### 8.3 构建与调试方式

开发环境已配置：

| 工具 | 方式 |
|------|------|
| VSCode API | 通过 API 读取文件、搜索、执行命令 |
| Go build | 直接调用 `go build ./...` |
| 静态检查 | `go vet ./...` |
| 运行测试 | `go test ./...` |
| 编译目标 | `go build -o build/echo-rebuild.exe .` |

每次修改后必须执行 `go build ./...` 确认编译通过，再提交。

### 8.4 模块完成标准

一个模块完成必须满足：

- [x] `go build ./...` 无报错
- [x] `go vet ./...` 无警告
- [x] 该模块导出的函数签名与 spec 一致
- [x] 边界情况有对应处理（空 slice、nil、空字符串）
- [x] 不包含调试输出/死代码/注释掉的代码
- [x] 实际耗时在超时容忍范围内

---

## 9. 实现顺序（按依赖链）

| 顺序 | 文件 | 原因 |
|------|------|------|
| 1 | `internal/store/models.go` | 零依赖，被所有包引用 |
| 2 | `internal/store/db.go` | 依赖 models，被 workflow 调用 |
| 3 | `internal/scanner/scanner.go` | 定义接口，依赖 models |
| 4 | `internal/scanner/scanner_windows.go` | 实现接口，依赖 scanner.go |
| 5 | `internal/scanner/scanner_unix.go` | 同上 |
| 6 | `internal/engine/pool.go` | 通用并发，零依赖 |
| 7 | `internal/engine/installer.go` | 依赖 models + pool（可选） |
| 8 | `internal/app/workflow.go` | 组装 scanner + store + engine + imageManager |
| 9 | `cmd/echo/styles.go` | 样式常量，零依赖 |
| 10 | `cmd/echo/shared/*.go` | 共享组件（输入框、磁盘列表、进度条） |
| 11 | `cmd/echo/pages/*.go` | TUI 各页面，依赖 shared 组件 |
| 12 | `cmd/echo/model.go` | 主 Model 状态机，路由到各页面 |
| 13 | `cmd/echo/main.go` | 入口，启动 program |
| 14 | `pkg/embed/assets.go` | 纯声明，最后补充 |
| 15 | `build.sh` | 不限顺序 |

## 9. 成功标准（验证清单）

```bash
# 验证 Windows 编译
$env:CGO_ENABLED="1"; go build -o build\echo-rebuild-windows-amd64.exe .\cmd\echo\

# 验证 Linux 交叉编译
$env:CGO_ENABLED="1"; $env:GOOS="linux"; $env:GOARCH="amd64"; go build -o build\echo-rebuild-linux-amd64 .\cmd\echo\

# 验证 macOS 交叉编译
$env:CGO_ENABLED="1"; $env:GOOS="darwin"; $env:GOARCH="amd64"; go build -o build\echo-rebuild-darwin-amd64 .\cmd\echo\
```

运行后验证：
1. 启动 → 主菜单显示（创建系统镜像 / 创建系统配置 / 还原系统配置 / 还原系统镜像 + 退出）
2. 选"创建系统镜像" → 进入 6 步向导（选类型→压缩→分卷→描述→选源盘→确认）
3. 选"还原系统镜像" → 进入 5 步向导（选文件→显示信息→选目标盘→修复引导→确认）
4. 进度页 → 实时显示进度、速度、用时
5. 完成页 → 显示结果汇总

---

## 10. TUI 交互界面设计

### 10.1 设计原则
- 全终端界面（TUI），无 GUI 窗口
- 键盘全操作：↑↓ 导航，数字键选择，Enter 确认，Esc 返回
- 输入框场景：自由文本输入（描述、路径等）
- 每个页面顶部固定标题，底部固定操作提示
- 返回上级统一用选项 `0`，放在所有菜单列表**最顶上**
- 所有选择项用单数字键触发（1、2、3…），不使用多键组合

### 10.2 页面结构树

```
主菜单
├── 1. 创建系统镜像
│       ├── 选择镜像类型
│       │    ├── 1. raw — 智能扇区复制
│       │    ├── 2. tbi — 块级镜像，支持增量/差分
│       │    └── 3. tar.zstd — 文件级归档，高压缩比
│       ├── 选择压缩等级 (各工具映射)
│       ├── 是否启用分卷/差分? [Y/n]
│       ├── 输入描述 (可选): 自由文本
│       ├── 选择源磁盘/分区
│       ├── 输入保存路径: 自由文本/浏览
│       ├── 确认页: 显示所有参数的摘要
│       └── 执行 → 进度页 → 完成页 → 返回
│
├── 2. 创建系统配置
│       ├── 扫描系统 (自动，显示进度)
│       ├── 配置分类树 (三级导航)
│       │    ├── 系统设置 (→展开子项勾选)
│       │    └── 软件 (平铺列表，按 p 设安装来源)
│       ├── 逐一设置安装来源 (URL / 免安装目录 / 压缩包)
│       ├── [Enter] 确认 → workflow.BackupConfig()
│       └── 完成页 → 返回
│
├── 3. 还原系统配置
│       ├── 选择 .db 文件 (输入路径/最近文件)
│       ├── 解析并展示分类树 (默认全选)
│       │    ├── 系统设置 (→展开子项取消勾选)
│       │    └── 软件 (平铺列表)
│       ├── [Enter] 确认恢复项 → 确认页
│       └── 执行 → 进度页 → 结果汇总 → 返回
│
└── 4. 还原系统镜像
        ├── 选择镜像文件 (输入路径或浏览)
        ├── 显示镜像信息 (类型/大小/描述/创建时间)
        ├── 选择目标磁盘/分区
        │    └── 列出所有可用磁盘 (列表选择)
        ├── 是否修复引导分区?
        │    ├── 1. 自动修复 (推荐)
        │    ├── 2. 手动指定引导分区 → 选择分区
        │    └── 3. 不修复
        ├── 什么是引导分区? → 展开说明弹窗 (按?查看)
        ├── 确认还原 → 确认页 (显示源/目标/选项)
        └── 执行 → 进度页 → 完成页 → 返回
```

### 10.3 主菜单（首页）

```
╔══════════════════════════════════════════╗
║          EchoRebuild v0.1.0              ║
║      跨平台系统备份 · 配置恢复工具        ║
╠══════════════════════════════════════════╣
║                                          ║
║   1.  创建系统镜像                        ║
║   2.  创建系统配置                        ║
║   3.  还原系统配置                        ║
║   4.  还原系统镜像                        ║
║                                          ║
║   0.  退出                               ║
║                                          ║
║  输入数字选择 [0-4]: _                    ║
╚══════════════════════════════════════════╝
```

> 排序逻辑：创建在前、还原在后；系统镜像在前、系统配置在后。
> 先备份（1、2）后还原（3、4），符合"先有备份才能还原"的自然流程。

### 10.4 创建系统镜像 — 完整交互流程

#### 10.4.1 选择镜像类型

```
╔══════════════════════════════════════════╗
║      选择镜像类型                         ║
╠══════════════════════════════════════════╣
║  0.  返回主菜单                          ║
║  1.  raw — 智能扇区复制，仅写已用空间     ║
║  2.  tbi — 块级镜像，支持增量/差分        ║
║  3.  tar.zstd — 文件级归档，高压缩比     ║
╚══════════════════════════════════════════╝
```

#### 10.4.2 压缩等级选择

```
  镜像类型: raw
  选择压缩等级:
  0.  返回上级
  1.  无压缩 (速度快，体积大)
  2.  快速压缩 (默认，均衡)
  3.  最大压缩 (速度慢，体积小)
  ─────────────────────────────────────
  (raw 不支持压缩，选项仅占位)
```

选 TBI 时同上 3 个选项，映射到 tbi 工具的 none / normal / high。

选 tar.zstd 时切换为数字输入：

```
  镜像类型: tar.zstd
  输入压缩等级 [0-19] (0=不压缩, 3=默认, 19=最小体积):
  > 6

  ─── 参考 ───
  0   不压缩（仅打包）
  1-3 快速压缩
  4-9 均衡（推荐 6）
  10+ 高压缩（慢）
```

#### 10.4.3 分卷/差分

```
  是否启用分卷?
  超过 4GB 自动分卷：RAW 按大小切 .img.001/.img.002…；TBI 内置支持；tar.zstd 流式切割
  [Y/n]: Y

  是否启用差分备份(基于上次全量)?
  [y/N]: N

  ─── 提示 ───
  差分仅 TBI 支持。tar.zstd 天然流式追加。RAW 不推荐差分。
```

#### 10.4.4 描述

```
   输入描述 (可选，Enter跳过):
   > Windows 10 Pro 22H2 完整工作站配置
```

#### 10.4.5 选择源磁盘

```
  选择要备份的源磁盘/分区:
  0.  返回上级
  1.  \\.\PhysicalDrive0  (256GB SSD)
      ├── C:  (237GB NTFS)  ← 系统盘 [当前选中]
      ├── 系统保留 (500MB FAT32)
      └── Recovery (800MB NTFS)
  2.  \\.\PhysicalDrive1  (1TB HDD)
      ├── D:  (500GB NTFS)
      └── E:  (464GB NTFS)
  ─────────────────────────────────────
  已选: PhysicalDrive0 全部
  按空格切换全盘/分区模式  Enter确认
```

#### 10.4.6 保存路径

```
  文件名 (可在预设基础上修改):
  > part_backup_20240617_143022

  路径: /backups/part_backup_20240617_143022.img
  可用空间: 256GB  |  预计镜像大小: ~15GB
  拖入目录自动填入路径  |  Enter确认  |  直接按Enter使用预设名
```

#### 10.4.7 确认页

```
╔══════════════════════════════════════════╗
║       确认备份参数                        ║
╠══════════════════════════════════════════╣
║                                          ║
║  镜像类型:    RAW                         ║
║  源设备:      /dev/nvme0n1 (512GB SSD)    ║
║  保存路径:    /backups/nvme0_20240815     ║
║  压缩等级:    不支持 (智能扇区复制)        ║
║  分卷:        是 (>4GB 自动分割)          ║
║  差分:        否                          ║
║  描述:        Arch Linux 工作站           ║

║                                          ║
║  1. 确认开始                              ║
║  0. 返回修改                              ║
╚══════════════════════════════════════════╝
```

### 10.5 还原系统镜像 — 完整交互流程

#### 10.5.1 选择镜像文件

```
  选择要还原的镜像文件:
  > /backups/ubuntu_20240815.tar.zst

  Tab键自动补全  |  Enter确认  |  拖入文件自动填入路径

  最近文件:
  1. /backups/ubuntu_20240815.tar.zst      (8.3GB)
  2. /backups/win10_20240701.tbi           (15.2GB)
  3. /backups/disk_backup.img.001          (6.8GB)
```

#### 10.5.2 镜像信息

```
╔══════════════════════════════════════════╗
║       镜像文件信息                        ║
╠══════════════════════════════════════════╣
║  文件名:    ubuntu_20240815.tar.zst       ║
║  类型:      tar.zstd                     ║
║  描述:      Ubuntu 24.04 工作站配置       ║
║            完整开发环境 + Docker 数据      ║
║  大小:      8.3 GB                       ║
║  创建时间:  2024-08-15 14:30:22          ║
║  加密:      否                            ║
║                                          ║
║  Enter继续  |  0. 返回选择                ║
╚══════════════════════════════════════════╝
```

#### 10.5.3 选择目标磁盘

```
  选择还原目标位置:
  0.  返回上级
  1.  \\.\PhysicalDrive0  (256GB SSD)
      ├── C:  (237GB NTFS)  ← 当前系统盘
      ├── 系统保留 (500MB FAT32)
      └── Recovery (800MB NTFS)
  2.  \\.\PhysicalDrive1  (1TB HDD)
      ├── D:  (500GB NTFS)  ← 可用空间充足
      └── E:  (464GB NTFS)
  ─────────────────────────────────────
  ⚠ 警告: 还原将覆盖目标磁盘上所有数据!
  按空格选择目标  Enter确认
```

#### 10.5.5 修复引导分区

```
  是否修复引导分区?

  1.  自动修复 (推荐)
       自动检测并修复 EFI 系统分区(ESP)或系统保留分区
  2.  手动指定引导分区
       自行选择用于存放引导文件的分区
  3.  不修复
       仅还原系统数据，引导保持不变

  0.  返回上级

  ─── 什么是引导分区? ───
  引导分区是存储启动所必需文件的分区。
   Windows: EFI 系统分区 (ESP, FAT32 ~100MB)
            或 系统保留分区 (NTFS ~500MB)
   Linux:   /boot 分区或 EFI 分区
   macOS:   EFI 系统分区
   修复 = 重新写入引导文件 + 配置启动项，
   确保还原后的系统能正常开机。
   Windows 用 bcdboot / bootrec
   Linux 用 grub-install / update-grub
   macOS 用 bless / diskutil

  按 ? 查看详细说明

  选择 [0-3]: 1
```

#### 10.5.6 确认还原

```
╔══════════════════════════════════════════╗
║       ⚠ 确认还原 — 此操作不可逆!         ║
╠══════════════════════════════════════════╣
║                                          ║
║  镜像文件:    ubuntu_20240815.tar.zst     ║
║  镜像大小:    8.3 GB                      ║
║  镜像类型:    tar.zstd                    ║
║  目标磁盘:    /dev/sda (256GB SSD)        ║
║  修复引导:    自动修复                    ║
║                                          ║
║  按 1 确认还原                            ║
║  按 0 取消返回                            ║
╚══════════════════════════════════════════╝
```

### 10.6 进度页（通用，镜像/配置共用）

```
╔══════════════════════════════════════════╗
║       正在创建系统镜像...                  ║
╠══════════════════════════════════════════╣
║                                          ║
║  ████████████████░░░░░░░░░░░░  58%       ║
║                                          ║
║  已处理:   148.2 GB / 256.0 GB           ║
║  速度:      125.6 MB/s                   ║
║  已用时:    00:02:15                     ║
║  预计剩余:  00:01:38                     ║
║                                          ║
║  当前操作:  raw — 智能扫描已用扇区...     ║
║                                          ║
║  按 Esc 取消 (将在当前文件完成后停止)      ║
╚══════════════════════════════════════════╝
```

### 10.7 完成页（通用）

```
╔══════════════════════════════════════════╗
║          操作完成                         ║
╠══════════════════════════════════════════╣
║                                          ║
║  ✓ 系统镜像创建成功                       ║
║                                          ║
║  文件:  /backups/nvme0_20240815.img.001  ║
║  大小:  12.5 GB                          ║
║  用时:  00:05:42                         ║
║                                          ║
║  Enter 返回主菜单                         ║
╚══════════════════════════════════════════╝
```

失败时显示：

```
╔══════════════════════════════════════════╗
║         操作失败                          ║
╠══════════════════════════════════════════╣
║                                          ║
║  ✗ 还原中断                              ║
║  错误: 目标磁盘 PhysicalDrive0 空间不足   ║
║  需要:  15GB  |  可用: 8GB               ║
║                                          ║
║  Enter 返回主菜单                         ║
╚══════════════════════════════════════════╝
```

### 10.8 创建系统配置 — 分类树交互

进入"创建系统配置"后，先自动扫描系统（显示进度），扫描完成后将结果组织为三级分类树，供用户导航勾选。

#### 10.8.1 三级分类树结构

```
等级1 (大类)         等级2 (子类)          等级3 (具体条目)
─────────────────────────────────────────────────────
软件（平铺列表，无子分类）
 ├── Firefox                                   [✓]
 ├── VS Code                                   [✓]
 ├── 7-Zip                                     [✓]
 ├── Docker Desktop                            [ ]
 ├── Python 3.13                               [✓]
 ├── Git                                       [✓]
 ├── OBS Studio                                [ ]
 ├── Node.js 22.x                              [✓]
 ├── Chrome                                    [ ]
 └── … (其余自动扫描到的软件)

系统设置
├── 系统环境变量                              [✓]
├── 时区/语言                                 [✓]
├── 网络配置                                  [ ]
├── 电源方案                                  [✓]
├── 桌面主题                                  [ ]
└── 用户账户                                  [✓]

电脑驱动
├── 显卡驱动                                  [✓]
├── 声卡驱动                                  [ ]
├── 网卡驱动                                  [✓]
├── 芯片组驱动                                [ ]
└── 打印机驱动                                [ ]
```

#### 10.8.2 交互操作

| 按键 | 行为 |
|------|------|
| `↑` `↓` | 在当前层级移动高亮行 |
| `→` | 进入高亮行的下一级（展开子类或条目列表） |
| `←` | 返回上一级 |
| `Space` | 切换当前行选中/取消：条目级切自身；分类级切整个分类 |
| `Enter` | 确认当前所有选中项，进入保存路径输入 |
| `Esc` | 返回主菜单 |

#### 10.8.3 界面布局

**一级界面**（刚进入时）：

```
 创建系统配置 — 选择要备份的内容             ↑↓移动  →展开  Space选择  Enter保存
┌────────────────────────────────────────────────────┐
│ [✓] 系统设置                                       │
│ [✓] 软件                              (10 项)    │
└────────────────────────────────────────────────────┘
  已选: 2/2 大类
```

**展开软件**（按→）：

```
 软件                                ← 按←返回上一级
┌────────────────────────────────────────────────────┐
│ [✓] Firefox                                       │
│ [✓] VS Code                                       │
│ [✓] 7-Zip                                         │
│ [ ] Docker Desktop                                │
│ [✓] Python 3.13                                   │
│ [✓] Git                                           │
│ [ ] OBS Studio                                    │
│ [✓] Node.js 22.x                                  │
│ [ ] Chrome                                        │
│ … (共 10 项)                                      │
└────────────────────────────────────────────────────┘
  已选: 6/10 项  |  Space切换  ↑↓滚动  Enter确认
```

**展开系统设置**（按→）：

```
 系统设置                              ← 按←返回上一级
┌────────────────────────────────────────────────────┐
│ [✓] 系统环境变量                                   │
│ [✓] 时区/语言                                      │
│ [ ] 网络配置                                       │
│ [✓] 电源方案                                       │
│ [ ] 桌面主题                                       │
│ [✓] 用户账户                                       │
└────────────────────────────────────────────────────┘
  已选: 4/6 项  |  Space切换  Enter确认此分类
```

#### 10.8.4 选中状态的传递规则

- 一级大类勾选 = 该分类下所有子项全选
- 二级分类勾选 = 该分类下所有条目全选
- 取消某子项 → 上级分类自动变为半选状态 `[~]`
- `Enter` 时收集所有被选中的叶子节点，传给 workflow.BackupConfig()

#### 10.8.5 逐一设置安装来源（手动）

选中某软件条目后按 `p` 键，进入该软件的安装来源设置：

```
  设置 "绿色工具"
  请选择安装包类型:
  1. URL 安装包 — 恢复时自动下载，失败则打开 URL
  2. 免安装目录 — 恢复时复制整个文件夹 + 创建桌面快捷方式
  3. 压缩包 — 恢复时打开路径让用户手动解压安装
  0. 跳过此软件（不备份安装包）
```

选 **1 (URL)**：

自动根据软件名称搜索官方下载页面（通过搜索引擎 API 或内置映射表）。用户可确认或修改搜索到的 URL：

```
   正在搜索 Firefox 官方下载地址...
   ✓ 已找到: https://www.mozilla.org/firefox/download/

   1. 确认使用此地址
   2. 修改地址
   0. 取消
```

选 2 修改则弹出输入框（同 §10.10 输入框组件规范），允许手动输入 URL。

选 **2 (免安装目录)**：

```
  输入文件夹相对路径（相对于备份根目录）:
  > Portable/GreenTool
  恢复时将: 复制 {根目录}/Portable/GreenTool → 目标位置 + 创建桌面快捷方式
  Enter确认
```

选 **3 (压缩包)**：

```
  输入压缩包相对路径（相对于备份根目录）:
  > Archives/GreenTool.7z
  恢复时将: 打开路径让用户手动处理
  Enter确认
```

填完后回到列表，该行显示已设置的来源类型：

```
┌──────┬─────────────────────────┬────────────────────────┐
│ 选中 │ 名称                    │ 安装包来源             │
├──────┼─────────────────────────┼────────────────────────┤
│ [✓]  │ Firefox                 │ URL                    │
│ [✓]  │ 绿色工具                │ 免安装 → Portable/...  │
│ [ ]  │ Docker                  │ (未设置)               │
│ [✓]  │ 某工具                  │ 压缩包 → Archives/...  │
└──────┴─────────────────────────┴────────────────────────┘
```

- 所有软件默认来源类型为 **URL**，扫描时自动搜索官方下载地址（§10.8.5）
- 用户可按 `p` 将特定条目改为「免安装目录」或「压缩包」
- 系统设置条目无来源概念
- 汇总显示：`N 项 (URL: X  免安装: Y  压缩包: Z)`

#### 10.8.6 确定保存

```
   即将备份以下内容:
     软件        — 10 项 (URL: 7  免安装: 2  压缩包: 1)
     系统设置    — 4 项
   ─────────────────────
   文件名 (可在预设基础上修改):
   > conf_backup_20240617_143022

   保存到: /backups/conf_backup_20240617_143022.db

   1. 确认保存
   0. 返回修改
```

---

### 10.9 还原系统配置 — 反向分类树

流程与创建系统配置对称：先选择 `.db` 文件，解析后展示同样的三级分类树，用户勾选要恢复的项后执行安装。

#### 10.9.1 交互流程

```
选择 .db 文件 (输入路径)
  ↓
解析数据库，按分类树重组 AppEntry
  ↓
展示分类树（同 10.8.1 结构，默认全部选中）
  │  ← / → / Space 操作与备份完全一致
  ↓
Enter → 确认恢复项 → 并行执行 → 进度页 → 结果汇总
```

#### 10.9.2 界面示例

**第一步：选择数据库**

```
  选择配置备份文件:
  > /backups/conf_backup_20240617_143022.db

  拖入文件自动填入路径  |  Enter确认

  最近文件:
  1. conf_backup_20240617_143022.db   (35 项)
  2. conf_backup_20240610_093015.db   (28 项)
```

**第二步：展示分类树（默认全选）**

```
 还原系统配置 — 选择要恢复的内容            ↑↓移动  →展开  Space取消  Enter恢复
┌────────────────────────────────────────────────────┐
│ [✓] 系统设置                            4/4 项    │
│ [✓] 软件                               6/10 项   │
└────────────────────────────────────────────────────┘
  已选: 10/14 项  |  按→展开细调  Enter开始恢复
```

展开某分类后操作与备份完全一致。

#### 10.9.3 执行恢复

确认后进入进度页（同 10.6），显示每项的安装进度，完成后跳转结果汇总页。

**结果汇总示例**：
```
操作完成 — 结果汇总

  ✓ 9 项成功
  ⚠ 2 项需手动操作:
       - 7-Zip (压缩包，路径已打开)
       - PortableApp (压缩包，路径已打开)
  ⚠ 1 项已回退到浏览器:
       - Docker Desktop (下载失败)

  Enter 返回
```

### 10.10 交互通用规则

| 场景 | 行为 |
|------|------|
| 菜单页 | 按数字键选择，Enter 无效 |
| 输入框 | 使用 `bubbletea/textinput` 组件，显示闪烁光标 `_`，Enter 提交，Esc 取消。所有路径输入框支持终端拖拽文件（Windows 终端拖入文件自动填入完整路径） |
| 确认页 | 1=确认，0=返回，其他键无反应 |
| 进度页 | 实时刷新，Esc 可取消 |
| 错误页 | 显示错误信息，Enter 返回 |
| 所有列表 | 0 始终在最顶上，表示返回 |
| 分类树（←→ Space） | → 展开子级，← 返回上级，Space 切换选中。系统设置始终在软件之上 |
| 软件列表（p） | 在软件条目上按 p，进入安装来源设置（URL/免安装/压缩包） |
| 结果汇总 | 除统计数字外，单独列出所有需要手动操作的条目名称 |
| 全局 | Esc 始终等同于"取消/返回"，不会意外退出程序 |

### 10.11 TUI 代码结构（`cmd/echo/` 目录内）

```
cmd/echo/
├── main.go                # program.Run() 入口
├── model.go               # 主 Model + state 分发
├── styles.go              # lipgloss 样式常量
├── types.go               # 共享类型 (页面ID, 选项等)
├── pages/
│   ├── menu.go             # 主菜单页面
│   │
│   ├── image_backup/
│   │   ├── select_type.go   # 选择镜像类型
│   │   ├── compression.go   # 选择压缩等级
│   │   ├── features.go      # 分卷/差分选择
│   │   ├── description.go   # 输入描述
│   │   ├── source_disk.go   # 选择源磁盘
│   │   ├── save_path.go     # 输入保存路径
│   │   └── confirm.go       # 确认页
│   │
│   ├── image_restore/
│   │   ├── select_file.go   # 选择镜像文件
│   │   ├── show_info.go     # 显示镜像信息
│   │   ├── target_disk.go   # 选择目标磁盘
│   │   ├── boot_repair.go   # 修复引导选项+说明
│   │   └── confirm.go       # 确认还原
│   │
│   ├── config_backup/
│   │   ├── tree.go           # 三级分类树页面（核心）
│   │   ├── scan_progress.go  # 扫描进度页
│   │   └── save_confirm.go   # 确认保存页
│   ├── config_restore/       # (预留)
│   ├── progress.go          # 进度页(通用)
│   └── result.go            # 完成/失败页(通用)
└── shared/
    ├── disk_list.go         # 磁盘列表组件(复用)
    ├── input.go             # 通用输入框组件
    └── progress_bar.go      # 进度条组件(复用)
```

**子 Model 接口约定**：
```go
type pageModel interface {
    Init() tea.Cmd
    Update(msg tea.Msg) (pageModel, tea.Cmd)
    View() string
}
```

主 Model 的 Update 中根据当前 `pageState` 枚举值路由到对应子 Model。
共享组件（磁盘列表、输入框、进度条）抽取到 `shared/` 目录复用。

**`shared/input.go` (输入框组件)**：
- 封装 `bubbletea/textinput`，统一所有输入场景：
  - `NewInput(placeholder, prompt string, width int) InputModel`
  - `SetValue(v string)` — 外部设置初始值
  - `Value() string` — 获取当前值
  - 支持自定义验证函数 `func(string) error`
- 拖拽文件支持：Windows 终端拖入文件时自动填入路径（监听 terminal paste event）
- 使用方式：
  ```go
  input := NewInput("输入路径...", "> ", 40)
  // 在 View() 中: input.View()
  // 在 Update() 中: input, cmd = input.Update(msg)
  // 提交: input.Value() != "" 且 msg 为 Enter 时确认
  ```
