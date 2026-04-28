# Windows 使用指南

本文档介绍如何在 Windows 平台上使用 backend-context-mcp。

## 系统要求

- Windows 10 或更高版本
- Go 1.16+ (如果从源码构建)
- PowerShell 5.1 或更高版本

## 安装方式

### 方式 1: 从 GitHub Releases 下载（推荐）

1. 访问 [Releases 页面](https://github.com/outhsics/backend-context-mcp/releases)
2. 下载 `backend-context-mcp-windows-amd64.exe`
3. 将文件重命名为 `backend-context-mcp.exe`
4. 将其放到系统 PATH 中的目录，如 `C:\Windows\System32\` 或用户目录

### 方式 2: 使用 npm

```cmd
npx backend-context-mcp --dir C:\path\to\backend
```

### 方式 3: 从源码构建

1. 克隆仓库：
```cmd
git clone https://github.com/outhsics/backend-context-mcp.git
cd backend-context-mcp
```

2. 使用 PowerShell 构建：
```powershell
.\build.ps1
```

或双击 `build.bat` 文件

3. 构建产物位于 `dist\backend-context-mcp-windows-amd64.exe`

## 使用方法

### 基本用法

```cmd
backend-context-mcp.exe --dir C:\path\to\backend
```

### 指定端口和主机

```cmd
REM 监听所有网络接口，端口 3100
backend-context-mcp.exe --dir C:\path\to\backend --host 0.0.0.0 --port 3100
```

### 使用配置文件

创建 `backend-context.config.json`：

```json
{
  "framework": "spring",
  "include": ["**/*.java"],
  "excludeDirs": [".git", "target", "build", ".gradle", "node_modules"],
  "controllerPaths": ["controller", "api", "resource", "web"],
  "dtoPaths": ["vo", "dto", "model", "request", "response", "payload"],
  "servicePaths": ["service", "application", "usecase"],
  "server": {
    "host": "127.0.0.1",
    "port": 3100
  }
}
```

然后运行：

```cmd
backend-context-mcp.exe --dir C:\path\to\backend --config backend-context.config.json
```

### 在 VS Code 中配置 MCP

在项目的 `.vscode/mcp.json` 中添加：

```json
{
  "mcpServers": {
    "backend-context": {
      "url": "http://127.0.0.1:3100/sse"
    }
  }
}
```

## 常见问题

### Q: 防火墙提示怎么办？
A: 首次运行时，Windows 可能会弹出防火墙提示。请允许此程序通过防火墙，以便 MCP 客户端能够连接。

### Q: 路径中的反斜杠需要转义吗？
A: 在命令行中，路径使用反斜杠 `\` 或正斜杠 `/` 都可以：
```cmd
backend-context-mcp.exe --dir C:\path\to\backend
backend-context-mcp.exe --dir C:/path/to/backend
```

在 JSON 配置文件中，需要转义反斜杠：
```json
{
  "excludeDirs": [".git", "target", "node_modules"]
}
```

### Q: 如何验证服务器是否正常运行？
A: 访问健康检查端点：
```powershell
Invoke-RestMethod -Uri http://127.0.0.1:3100/health
```

或使用浏览器打开：`http://127.0.0.1:3100/health`

### Q: PowerShell 执行策略错误
A: 如果看到 "cannot be loaded because running scripts is disabled on this system" 错误：

```powershell
# 临时允许（推荐）
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass

# 然后运行
.\build.ps1
```

### Q: 端口被占用
A: 使用 `--port auto` 自动选择可用端口：
```cmd
backend-context-mcp.exe --dir C:\path\to\backend --port auto
```

或设置环境变量：
```cmd
set PORT=3200
backend-context-mcp.exe --dir C:\path\to\backend
```

## 性能优化

对于大型项目，可以调整以下参数：

1. **增加 Java 文件扫描限制**：修改代码中的 `excludeDirs` 配置
2. **使用 SSD**：确保项目在固态硬盘上，扫描速度会快得多
3. **启用缓存**：服务器会缓存扫描结果，修改代码后调用 `refresh_cache` 工具刷新

## 开发模式

如需允许 MCP 客户端读取 Service 源码：

```cmd
backend-context-mcp.exe --dir C:\path\to\backend --allow-source-tools
```

**注意**：仅在受信任的网络环境中启用此功能。

## 卸载

1. 删除可执行文件
2. 删除配置文件（如果有）
3. 删除项目中的 `.vscode/mcp.json` 配置（如果不再需要）

## 技术支持

如遇到问题，请：
1. 查看上述常见问题
2. 检查 [GitHub Issues](https://github.com/outhsics/backend-context-mcp/issues)
3. 提交新的 Issue，包含：
   - Windows 版本
   - 错误信息
   - 复现步骤
   - 配置文件（去除敏感信息）
