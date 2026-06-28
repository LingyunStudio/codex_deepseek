package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"codeseek/internal/config"
	"codeseek/internal/extension/codex"
	"codeseek/internal/logger"
	"codeseek/internal/service/app"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed frontend
var frontendAssets embed.FS

//go:embed config.example.yml
var configTemplate string

// ── LogRing ────────────────────────────────────────────────────────────────

type LogRing struct {
	mu   sync.Mutex
	buf  []string
	pos  int
	full bool
}

func NewLogRing(size int) *LogRing { return &LogRing{buf: make([]string, size)} }

func (r *LogRing) Write(p []byte) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, line := range strings.Split(strings.TrimRight(string(p), "\n\r"), "\n") {
		if line == "" {
			continue
		}
		r.buf[r.pos] = line
		r.pos++
		if r.pos >= len(r.buf) {
			r.pos = 0
			r.full = true
		}
	}
	return len(p), nil
}

func (r *LogRing) Lines() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.full {
		out := make([]string, r.pos)
		copy(out, r.buf[:r.pos])
		return out
	}
	out := make([]string, 0, len(r.buf))
	out = append(out, r.buf[r.pos:]...)
	out = append(out, r.buf[:r.pos]...)
	return out
}

func (r *LogRing) Tail(n int) []string {
	lines := r.Lines()
	if len(lines) <= n {
		return lines
	}
	return lines[len(lines)-n:]
}

// ── App ────────────────────────────────────────────────────────────────────

type App struct {
	mu           sync.Mutex
	configPath   string
	cfg          *config.Config
	port         string
	running      bool
	serverCtx    context.Context
	serverStop   context.CancelFunc
	logRing      *LogRing
	activeModel  string
	codexHome    string
}

func (a *App) ServiceName() string { return "CodeSeek" }

func (a *App) ServiceStartup(_ context.Context, _ application.ServiceOptions) error {
	a.logRing = NewLogRing(500)
	home, _ := os.UserHomeDir()
	a.codexHome = filepath.Join(home, ".codex")
	return nil
}

func (a *App) ServiceShutdown() error {
	a.StopServer()
	return nil
}

// ── Config Path ────────────────────────────────────────────────────────────

func (a *App) ensureWritableConfig() {
	if a.configPath == "" || !isSystemPath(a.configPath) {
		return
	}
	appdata := os.Getenv("APPDATA")
	if appdata == "" { return }
	writablePath := filepath.Join(appdata, "codeseek", "config.yml")
	if _, err := os.Stat(writablePath); err == nil {
		a.configPath = writablePath
		return
	}
	if data, err := os.ReadFile(a.configPath); err == nil {
		os.MkdirAll(filepath.Dir(writablePath), 0755)
		if os.WriteFile(writablePath, data, 0644) == nil {
			a.configPath = writablePath
		}
	}
}

// ── Config I/O ─────────────────────────────────────────────────────────────

func (a *App) LoadConfig() (map[string]any, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.configPath == "" {
		// Try existing config locations first.
		a.configPath = resolveDefaultConfigPath()
		if a.configPath == "" {
			// Neither existing file nor XDG fallback — compute default and create.
			if p, err := defaultConfigPath(); err == nil {
				if _, e := os.Stat(p); os.IsNotExist(e) {
					if err := os.MkdirAll(filepath.Dir(p), 0755); err == nil {
						os.WriteFile(p, []byte(configTemplate), 0644)
					}
				}
				// Use default path regardless (it may have just been created).
				a.configPath = p
			}
		}
		if a.configPath == "" {
			return nil, fmt.Errorf(configPathNotFoundMessage())
		}
	}

	cfg, err := config.LoadFromFileWithOptions(a.configPath, config.LoadOptions{
		ExtensionSpecs: app.BuiltinExtensions().ConfigSpecs(),
	})
	if err != nil {
		return nil, fmt.Errorf("加载配置失败: %w", err)
	}
	a.cfg = &cfg
	a.port = parsePort(cfg.Addr)
	if a.activeModel == "" {
		a.activeModel = cfg.DefaultModelAlias()
	}
	// Ensure we're using a writable copy (e.g. %APPDATA%).
	a.ensureWritableConfig()
	return configToMap(cfg, a.configPath, a.GetActiveModel()), nil
}

func (a *App) GetConfigPath() string { return a.configPath }

func (a *App) SetConfigPath(path string) { a.configPath = path }

// ── Server ─────────────────────────────────────────────────────────────────

func (a *App) StartServer() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.running {
		return fmt.Errorf("服务已在运行")
	}
	if a.cfg == nil {
		return fmt.Errorf("请先加载配置")
	}
	cfgCopy := *a.cfg
	// Ensure SQLite path is absolute and writable from any working directory.
	home, _ := os.UserHomeDir()
	if cfgCopy.Extensions != nil {
		if _, ok := cfgCopy.Extensions["db_sqlite"]; ok && cfgCopy.Extensions["db_sqlite"].RawConfig != nil {
			raw := cfgCopy.Extensions["db_sqlite"].RawConfig
			if path, ok := raw["path"].(string); ok {
				if filepath.IsAbs(path) {
					os.MkdirAll(filepath.Dir(path), 0755)
				} else {
					dataDir := filepath.Join(home, ".codeseek", "data")
					os.MkdirAll(dataDir, 0755)
					raw["path"] = filepath.Join(dataDir, filepath.Base(path))
				}
			}
		}
	}
	// Auto-backup and enable Codex proxy.
	a.ensureCodexBackupAndEnable()
	ctx, cancel := context.WithCancel(context.Background())
	a.serverCtx = ctx
	a.serverStop = cancel
	logger.Init(logger.Config{Level: "info", Format: "text", Output: io.MultiWriter(a.logRing, os.Stderr)})
	go func() {
		a.logRing.Write([]byte(fmt.Sprintf("CodeSeek 启动中，监听 %s ...", cfgCopy.Addr)))
		if err := app.RunServer(ctx, cfgCopy, io.MultiWriter(a.logRing, os.Stderr)); err != nil {
			a.logRing.Write([]byte(fmt.Sprintf("服务错误: %v", err)))
		}
	}()
	time.Sleep(300 * time.Millisecond)
	a.running = true
	return nil
}

func (a *App) StopServer() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.running {
		return nil
	}
	if a.serverStop != nil {
	a.disableCodexProxyIfEnabled()
		a.serverStop()
	}
	a.running = false
	a.serverCtx = nil
	a.serverStop = nil
	return nil
}

func (a *App) IsRunning() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.running
}

// ── Logs ──────────────────────────────────────────────────────────────────

func (a *App) GetLogs() []string {
	if a.logRing == nil {
		return nil
	}
	return a.logRing.Tail(200)
}

// ── Model Selection ────────────────────────────────────────────────────────

func (a *App) GetModels() []string {
	if a.cfg == nil {
		return nil
	}
	// Only return models that have at least one provider offer.
	hasOffer := make(map[string]bool)
	for _, def := range a.cfg.ProviderDefs {
		for _, offer := range def.Offers {
			hasOffer[offer.Model] = true
		}
	}
	var names []string
	for slug := range a.cfg.Models {
		if hasOffer[slug] {
			names = append(names, slug)
		}
	}
	sort.Strings(names)
	return names
}

func (a *App) GetActiveModel() string {
	if a.activeModel == "" {
		// Return the upstream model from the first route.
		for _, route := range a.cfg.Routes {
			return route.Model
		}
		models := a.GetModels()
		if len(models) > 0 {
			return models[0]
		}
		return "deepseek-v4-pro"
	}
	// If activeModel is a route alias, return the underlying model.
	if route, ok := a.cfg.Routes[a.activeModel]; ok && route.Model != "" {
		return route.Model
	}
	return a.activeModel
}

func (a *App) SetActiveModel(model string) {
	a.activeModel = model
	// Update all routes pointing to the current active model's route to use the new model.
	if a.cfg != nil && a.cfg.Routes != nil {
		for alias, route := range a.cfg.Routes {
			if provider, ok := a.cfg.ProviderDefs[route.Provider]; ok {
				if _, ok := provider.Models[model]; ok {
					route.Model = model
					a.cfg.Routes[alias] = route
				}
			}
		}
	}
}

// ── API Key ───────────────────────────────────────────────────────────────

func (a *App) GetAPIKey(provider string) string {
	if a.cfg == nil {
		return ""
	}
	if def, ok := a.cfg.ProviderDefs[provider]; ok {
		return def.APIKey
	}
	return ""
}

func (a *App) SetAPIKey(provider, key string) error {
	if a.cfg == nil {
		return fmt.Errorf("请先加载配置")
	}
	def, ok := a.cfg.ProviderDefs[provider]
	if !ok {
		return fmt.Errorf("provider %q 不存在", provider)
	}
	def.APIKey = key
	a.cfg.ProviderDefs[provider] = def

	// Write back to config.yml.
	if a.configPath != "" {
		data, err := os.ReadFile(a.configPath)
		if err != nil {
			return fmt.Errorf("读取配置文件失败: %w", err)
		}
		lines := strings.Split(string(data), "\n")
		inProvider := false
		found := false
		providerIndent := ""
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == provider+":" {
				inProvider = true
				// Detect indentation from this line
				providerIndent = line[:len(line)-len(trimmed)]
				continue
			}
			if inProvider {
				if strings.HasPrefix(trimmed, "api_key:") {
					indent := line[:len(line)-len(trimmed)]
					lines[i] = indent + "api_key: \"" + key + "\""
					found = true
					break
				}
				// If we hit another top-level key, we're past the provider block
				if trimmed != "" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
					break
				}
			}
		}
		if !found && providerIndent != "" {
			// API key line not found — insert it after the provider name line.
			for i, line := range lines {
				if strings.TrimSpace(line) == provider+":" {
					lines[i] = line + "\n" + providerIndent + "  api_key: \"" + key + "\""
					break
				}
			}
		}
		newData := strings.Join(lines, "\n")
		if err := os.WriteFile(a.configPath, []byte(newData), 0644); err != nil {
			return fmt.Errorf("写入配置文件失败: %w", err)
		}
	}
	return nil
}

func (a *App) GetProviderName() string {
	if a.cfg == nil {
		return ""
	}
	// Return the first provider name from routes
	for _, route := range a.cfg.Routes {
		return route.Provider
	}
	for key := range a.cfg.ProviderDefs {
		return key
	}
	return ""
}

// ── Port ──────────────────────────────────────────────────────────────────

func (a *App) GetPort() string {
	if a.port == "" {
		return "38440"
	}
	return a.port
}

func (a *App) SetPort(port string) { a.port = port }

// ── Codex Config ──────────────────────────────────────────────────────────

func (a *App) GetCodexHome() string { return a.codexHome }

func (a *App) BackupCodexConfig() error {
	src := filepath.Join(a.codexHome, "config.toml")
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return fmt.Errorf("未找到 Codex 配置文件: %s", src)
	}
	dst := src + ".codeseek.bak"
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

func (a *App) RestoreCodexConfig() error {
	dst := filepath.Join(a.codexHome, "config.toml")
	src := dst + ".codeseek.bak"
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return fmt.Errorf("未找到备份文件，请先备份")
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// EnsureCodexBackup creates a backup of Codex config if one doesn't exist.
func (a *App) EnsureCodexBackup() (bool, error) {
	src := filepath.Join(a.codexHome, "config.toml")
	dst := src + ".codeseek.bak"
	if _, err := os.Stat(dst); err == nil {
		return false, nil // already backed up
	}
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return false, nil // no Codex config to back up
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return false, err
	}
	return true, os.WriteFile(dst, data, 0644)
}

// EnableCodexProxy injects openai_base_url into the Codex config.
func (a *App) EnableCodexProxy() error {
	path := filepath.Join(a.codexHome, "config.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content := string(data)
	baseURL := "http://127.0.0.1:" + a.GetPort() + "/v1"
	modelName := a.GetActiveModel()
	if modelName == "" {
		modelName = "codeseek"
	}
	line := "openai_base_url = \"" + baseURL + "\""
	modelLine := "model = \"" + modelName + "\""
	catalogLine := "model_catalog_json = \"" + strings.ReplaceAll(filepath.Join(a.codexHome, "models_catalog.json"), "\\", "\\\\") + "\""

	if strings.Contains(content, "openai_base_url") {
		lines := strings.Split(content, "\n")
		for i, l := range lines {
			if strings.HasPrefix(strings.TrimSpace(l), "openai_base_url") {
				lines[i] = line
				break
			}
		}
		// Also replace or inject model line.
		hasModel := false
		for i, l := range lines {
			// Exact key match: only "model =", not "model_catalog_json =" etc.
			parts := strings.SplitN(strings.TrimSpace(l), "=", 2)
			if len(parts) == 2 && strings.TrimSpace(parts[0]) == "model" {
				lines[i] = modelLine
				hasModel = true
				break
			}
		}
		if !hasModel {
			for i, l := range lines {
				if strings.HasPrefix(strings.TrimSpace(l), "openai_base_url") {
					lines = append(lines[:i+1], append([]string{modelLine}, lines[i+1:]...)...)
					break
				}
			}
		}
		content = strings.Join(lines, "\n")
	} else {
		// Remove any existing model = line to avoid duplicates.
		{
			lines := strings.Split(content, "\n")
			var filtered []string
			for _, l := range lines {
				parts := strings.SplitN(strings.TrimSpace(l), "=", 2)
				if len(parts) == 2 && strings.TrimSpace(parts[0]) == "model" {
					continue
				}
				filtered = append(filtered, l)
			}
			content = strings.Join(filtered, "\n")
		}
		if idx := strings.Index(content, "[openai]"); idx >= 0 {
			end := strings.Index(content[idx:], "\n[")
			if end < 0 { end = len(content[idx:]) }
			content = content[:idx+end] + "\n" + line + "\n" + modelLine + "\n" + catalogLine + content[idx+end:]
		} else {
			content = line + "\n" + modelLine + "\n" + catalogLine + "\n" + content
		}
	}

	// Ensure model_catalog_json is present
	if !strings.Contains(content, "model_catalog_json") {
		content = strings.Replace(content, line, line+"\n"+catalogLine, 1)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return err
	}

	// Write models_catalog.json using the built-in generator, then filter out
	// route aliases (only keep models with provider offers).
	if a.cfg != nil {
		providerCfg := config.ProviderFromGlobalConfig(a.cfg)
		pluginCfg := config.PluginFromGlobalConfig(a.cfg)
		catalogPath := filepath.Join(a.codexHome, "models_catalog.json")
		if err := codex.WriteModelsCatalog(catalogPath, providerCfg, pluginCfg); err != nil {
			a.logRing.Write([]byte(fmt.Sprintf("models_catalog.json: %v", err)))
		} else {
			// Filter out route aliases — only keep real model slugs with offers.
			filterCatalogToOffers(catalogPath, a.cfg)
		}
	}

	return nil
}

func filterCatalogToOffers(path string, cfg *config.Config) {
	data, err := os.ReadFile(path)
	if err != nil { return }
	var catalog struct {
		Models []map[string]any `json:"models"`
	}
	if json.Unmarshal(data, &catalog) != nil { return }
	offered := make(map[string]bool)
	for _, def := range cfg.ProviderDefs {
		for _, offer := range def.Offers {
			offered[offer.Model] = true
		}
	}
	filtered := make([]map[string]any, 0, len(catalog.Models))
	for _, m := range catalog.Models {
		if slug, ok := m["slug"].(string); ok && offered[slug] {
			filtered = append(filtered, m)
		}
	}
	catalog.Models = filtered
	newData, _ := json.MarshalIndent(catalog, "", "  ")
	os.WriteFile(path, newData, 0644)
}

// DisableCodexProxy restores the Codex config from backup.
func (a *App) DisableCodexProxy() error {
	return a.RestoreCodexConfig()
}

// IsCodexRunning checks if any codex process is running.
func (a *App) IsCodexRunning() bool {
	return checkCodexProcess()
}

func (a *App) ensureCodexBackupAndEnable() {
	backedUp, err := a.EnsureCodexBackup()
	if err != nil {
		a.logRing.Write([]byte(fmt.Sprintf("Codex 备份失败: %v", err)))
		return
	}
	if backedUp {
		a.logRing.Write([]byte("Codex 配置已备份"))
	}
	if err := a.EnableCodexProxy(); err != nil {
		a.logRing.Write([]byte(fmt.Sprintf("Codex 代理配置失败: %v", err)))
	} else {
		a.logRing.Write([]byte("Codex 代理已启用"))
	}
}

func (a *App) disableCodexProxyIfEnabled() {
	if err := a.DisableCodexProxy(); err != nil {
		a.logRing.Write([]byte(fmt.Sprintf("Codex 配置恢复失败: %v (可手动恢复)")))
	} else {
		a.logRing.Write([]byte("Codex 配置已恢复"))
	}
}

// ── Helpers ────────────────────────────────────────────────────────────────

func parsePort(addr string) string {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[i+1:]
		}
	}
	return ""
}

func configToMap(cfg config.Config, configPath, activeModel string) map[string]any {
	providers := make([]map[string]any, 0)
	providerNames := make([]string, 0)
	for key, def := range cfg.ProviderDefs {
		modelNames := make([]string, 0, len(def.Models))
		for name := range def.Models {
			modelNames = append(modelNames, name)
		}
		providers = append(providers, map[string]any{
			"key":        key,
			"base_url":   def.BaseURL,
			"protocol":   valueOrDefault(def.Protocol, "anthropic"),
			"models":     modelNames,
			"user_agent": def.UserAgent,
			"version":    def.Version,
		})
		providerNames = append(providerNames, key)
	}
	routes := make([]map[string]any, 0)
	for alias, route := range cfg.Routes {
		routes = append(routes, map[string]any{"alias": alias, "provider": route.Provider, "model": route.Model})
	}
	modelsSlice := make([]map[string]any, 0)
	modelNames := make([]string, 0)
	// Only include models that have at least one provider offer.
	hasOffer := make(map[string]bool)
	for _, def := range cfg.ProviderDefs {
		for _, offer := range def.Offers {
			hasOffer[offer.Model] = true
		}
	}
	for slug, def := range cfg.Models {
		if !hasOffer[slug] {
			continue
		}
		modelsSlice = append(modelsSlice, map[string]any{
			"slug":                        slug,
			"display_name":                def.DisplayName,
			"context_window":              def.ContextWindow,
			"max_output_tokens":           def.MaxOutputTokens,
			"default_reasoning_level":     def.DefaultReasoningLevel,
			"supports_reasoning_summaries": def.SupportsReasoningSummaries,
		})
		modelNames = append(modelNames, slug)
	}
	return map[string]any{
		"mode":         string(cfg.Mode),
		"addr":         cfg.Addr,
		"log_level":    cfg.LogLevel,
		"log_format":   cfg.LogFormat,
		"config_path":  configPath,
		"active_model": activeModel,
		"model_names":  modelNames,
		"provider_names": providerNames,
		"providers":    providers,
		"routes":       routes,
		"models":       modelsSlice,
		"cache":        map[string]any{"mode": cfg.Cache.Mode, "ttl": cfg.Cache.TTL},
		"web_search":   map[string]any{"support": string(cfg.WebSearchSupport), "tavily_key": cfg.TavilyAPIKey},
		"extensions":   cfg.Extensions,
	}
}

func valueOrDefault(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

// ── Entry Point ────────────────────────────────────────────────────────────

func main() {
	sub, err := fs.Sub(frontendAssets, "frontend")
	if err != nil {
		fmt.Fprintln(os.Stderr, "加载前端资源失败:", err)
		os.Exit(1)
	}

	guiApp := &App{}

	// Load app icon for Windows (embedded resource).
	appIconData, _ := fs.ReadFile(sub, "src/icon-app.png")

	wailsApp := application.New(application.Options{
		Name:        "CodeSeek",
		Description: "LLM API 协议转换与模型路由代理",
		Icon:        appIconData,
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(sub),
		},
		Mac: application.MacOptions{
			ActivationPolicy: application.ActivationPolicyAccessory,
		},
		Services: []application.Service{
			application.NewService(guiApp),
		},
	})
	// Load icons from embedded assets.

	win := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:  "main",
		Title: "CodeSeek",
		Width: 1100,
		Height: 720,
	})

	// ── System Tray ──
	tray := wailsApp.SystemTray.New()
	tray.SetTooltip("CodeSeek - 协议转换代理")

	trayIcon, _ := fs.ReadFile(sub, "src/icon-tray.png")
	if len(trayIcon) > 0 {
		tray.SetIcon(trayIcon)
	}

	// Tray click → show window.
	tray.OnClick(func() {
		if win.IsMinimised() {
			win.UnMinimise()
		}
		win.Show()
		win.Focus()
	})

	// Tray right-click menu.
	trayMenu := wailsApp.Menu.New()
	trayMenu.Add("显示主窗口").OnClick(func(_ *application.Context) {
		win.Show()
		win.Focus()
	})
	trayMenu.Add("退出 CodeSeek").OnClick(func(_ *application.Context) {
		guiApp.StopServer()
		wailsApp.Quit()
	})
	tray.SetMenu(trayMenu)

	// Intercept window close → hide to tray instead of quitting.
	win.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		win.Hide()
		e.Cancel()
	})

	if err := wailsApp.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "CodeSeek GUI 启动失败:", err)
		os.Exit(1)
	}
}

