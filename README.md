# CodeSeek

让 [Codex CLI](https://github.com/anthropics/codex) 使用 DeepSeek 模型的协议转换代理。

**工作原理：** CodeSeek 在本地启动一个 HTTP 服务（`127.0.0.1:38440`），将 Codex 发出的 OpenAI Responses API 请求转换为 Anthropic Messages API 格式，转发到 DeepSeek，返回响应时再转换回 OpenAI 格式。带桌面 GUI 管理界面。

```
Codex CLI (OpenAI Responses API)
    │
    ▼
CodeSeek (127.0.0.1:38440)
    │ 协议转换 + 模型路由
    ▼
DeepSeek API (Anthropic Messages 兼容)
```

---

## 安装

### macOS

1. 下载 `CodeSeek.dmg`，双击打开
2. 将 `CodeSeek.app` 拖入 `Applications` 文件夹
3. 首次启动时会自动创建配置文件（路径见下方）
4. 填入 DeepSeek API Key → 保存 → 点击 "启动服务"

### Windows

1. 下载 `CodeSeek-Setup-1.0.0.exe`，双击安装
2. 启动 CodeSeek → 填入 DeepSeek API Key → 保存
3. 点击 "启动服务" → 自动配置 Codex

---

## 配置文件

程序首次启动时自动在以下位置创建模板：

| 平台 | 路径 |
|------|------|
| macOS | `~/Library/Application Support/codeseek/config.yml` |
| Windows | `%APPDATA%/codeseek/config.yml` |
| Linux | `~/.config/codeseek/config.yml` |

数据库文件：`~/.codeseek/data/codeseek.db`

### 配置示例

```yaml
mode: Transform
server:
  addr: "127.0.0.1:38440"

models:
  deepseek-v4-pro:
    context_window: 1000000
    max_output_tokens: 384000
    extensions:
      deepseek_v4:
        enabled: true

providers:
  deepseek:
    base_url: "https://api.deepseek.com/anthropic"
    api_key: "你的 API Key"
    protocol: anthropic
    offers:
      - model: deepseek-v4-pro

routes:
  codeseek:
    model: deepseek-v4-pro
    provider: deepseek
```

完整配置说明见 [docs/CONFIGURATION.md](docs/CONFIGURATION.md)。

---

## 开发

### 环境要求

- Go 1.25+
- Node.js + pnpm（仅 GUI 构建需要）
- macOS: Xcode Command Line Tools
- Windows: MinGW-w64（编译图标资源需要）

### 安装工具链

```bash
# Go
brew install go           # macOS
# 或从 https://go.dev/dl/ 下载安装包

# Wails v3 CLI
go install github.com/wailsapp/wails/v3/cmd/wails3@latest

# pnpm
npm install -g pnpm
```

### 构建

```bash
# ── 所有平台 ──
make cli              # 仅构建 CLI 二进制

# ── macOS ──
make gui              # 仅构建 GUI 二进制
make dmg              # 构建 GUI + DMG 安装包
./mac.sh all          # CLI + GUI + DMG 一键构建
./mac.sh clean        # 清理构建产物

# ── Windows ──
powershell -File build.ps1     # 构建 GUI（需要修改里面的Inno Setup 路径）
powershell -File package.ps1   # 打包安装程序（需 Inno Setup）
```

### 运行测试

```bash
make test             # 运行所有测试
make cover            # 查看覆盖率
```

### CLI 模式

```bash
./codeseek -config config.example.yml
```

### CLI 参数

| 参数 | 说明 |
|------|------|
| `-config <path>` | 配置文件路径 |
| `-addr <addr>` | 覆盖监听地址 |
| `-mode <mode>` | 覆盖运行模式（Transform / CaptureAnthropic / CaptureResponse） |
| `-print-codex-config <model>` | 生成 Codex config.toml 并输出到 stdout |
| `-codex-home <dir>` | Codex 配置目录（默认 `~/.codex`） |
| `-dump-config-schema` | 生成 JSON Schema |

---

## 功能

- 协议转换：OpenAI Responses API ↔ Anthropic Messages API
- 模型路由：支持多 provider、多模型切换
- DeepSeek V4 适配：reasoning_content 处理、thinking block 注入
- 桌面 GUI：启动/停止服务、API Key 管理、日志查看、深浅色主题
- Codex 自动配置：自动注入 base_url 和 model catalog，停止时恢复

---

## Docker

```bash
docker build -t codeseek .
docker run -p 38440:38440 -v $(pwd)/config.yml:/config/config.yml codeseek
```

---

## 许可证

[GPL-3.0](LICENSE)
