# Backend Context MCP Server

> 前端联调神器 — 一个二进制文件，后端同事运行后，前端 AI 助手自动获取后端代码上下文

## 解决什么问题？

前后端联调时，前端开发者经常需要问后端同事：
- 这个接口要传什么参数？
- 返回的 VO 里有哪些字段？
- 这个接口有没有权限校验？
- 为什么返回了 403？

这个工具让这些信息直接被 AI 助手（Cursor / Claude Code / VS Code Copilot）获取，**不需要后端同事每次手动回答**。

## 工作原理

```
后端同事电脑                          前端开发者
┌──────────────────┐                 ┌──────────────────┐
│  Java 后端项目    │                 │  前端项目         │
│  (Spring Cloud)  │                 │  (Vue/React)     │
│       │          │                 │       │          │
│  二进制文件扫描   │                 │  AI 助手         │
│  代码 → 提取上下文│   ←── HTTP ──→  │  (Cursor 等)     │
│       │          │    局域网        │       │          │
│  MCP Server      │                 │  MCP Client      │
│  (:3100)         │                 │                  │
└──────────────────┘                 └──────────────────┘
```

1. 后端同事运行一个二进制文件（零安装、零配置）
2. 工具扫描后端 Java 代码，提取 Controller、VO、Service 信息
3. 前端通过 MCP 协议（HTTP SSE）远程访问这些信息
4. AI 助手自动调用工具，获取后端上下文

## 快速开始

### 第 1 步：后端同事启动服务

下载对应平台的二进制文件（[Releases](../../releases)），然后：

```bash
# macOS / Linux
chmod +x byjyedu-backend-context
./byjyedu-backend-context --dir /path/to/your/java/backend/project

# Windows
byjyedu-backend-context.exe --dir C:\path\to\your\java\backend\project
```

看到以下输出就成功了：
```
扫描完成: 1020 个路由, 1352 个 VO
🚀 Backend Context MCP Server 已启动
   后端代码: /path/to/your/backend/project
   SSE 端点: http://192.168.1.100:3100/sse
   健康检查: http://192.168.1.100:3100/health
```

把这个 SSE 地址告诉前端开发者。

### 第 2 步：前端开发者配置 AI 助手

**Cursor / VS Code + Claude：**

在前端项目根目录创建或编辑 `.vscode/mcp.json`：

```json
{
  "mcpServers": {
    "backend-context": {
      "url": "http://后端同事IP:3100/sse"
    }
  }
}
```

**Claude Code：**

```bash
claude mcp add backend-context --transport sse http://后端同事IP:3100/sse
```

### 第 3 步：直接问 AI

配置好后，直接在 AI 助手中用自然语言对话：

```
你: 后端有哪些会议相关的接口？
AI: (自动调用 list_routes，返回路由表)

你: /edu-teaching/meeting/page 这个接口怎么调？
AI: (自动调用 get_api_detail，返回完整参数和 VO 字段)

你: MeetingCreateReqVO 有哪些字段？
AI: (自动调用 search_vo，返回字段定义)

你: MeetingService 创建会议的逻辑是什么？
AI: (自动调用 get_service_logic，返回 Service 源码)
```

## 命令行参数

```bash
./byjyedu-backend-context [选项]

选项:
  --dir string    后端项目根目录路径 (默认: 当前目录)
  --port int      服务端口 (默认: 3100)
  --host string   监听地址 (默认: 0.0.0.0)
```

示例：

```bash
# 指定后端项目路径和端口
./byjyedu-backend-context --dir ~/projects/my-backend --port 8080

# 在 Windows 上
byjyedu-backend-context.exe --dir D:\projects\my-backend --port 8080
```

## AI 工具列表

| 工具名 | 说明 | 使用场景 |
|--------|------|----------|
| `list_routes` | 列出后端所有 API 路由，支持关键词筛选 | "有哪些接口？" |
| `get_api_detail` | 获取指定接口的完整上下文：参数、返回值、VO 字段 | "这个接口怎么调？" |
| `search_vo` | 搜索 VO/DTO 类的字段定义 | "这个字段是什么类型？" |
| `get_service_logic` | 读取 Service 层源码，理解业务逻辑 | "为什么返回这个错误？" |
| `refresh_cache` | 刷新代码缓存 | "后端改了代码，更新一下" |

## 支持的后端框架

基于 Java 注解解析，支持以下 Spring 生态框架：

- Spring Boot / Spring Cloud
- Spring MVC (`@RestController`, `@RequestMapping`, `@GetMapping` 等)
- Swagger / OpenAPI (`@Tag`, `@Operation`, `@Schema`, `@Parameter`)
- Spring Security (`@PreAuthorize`)
- Jakarta Validation (`@Valid`, `@Validated`)
- Lombok (`@Data` 生成的字段通过源码解析)

自动识别的代码结构：

```
后端项目/
├── controller/        → 提取路由、HTTP 方法、参数、权限
│   ├── admin/         → 管理后台接口
│   └── app/           → App 端接口
├── vo/                → 提取请求/响应 VO 字段定义
└── service/           → 提供源码查看能力
```

## 从源码编译

只需要有 Go 环境（仅编译时需要，使用者不需要）：

```bash
# 克隆项目
git clone https://github.com/your-username/backend-context-mcp.git
cd backend-context-mcp

# 直接编译当前平台
go build -o byjyedu-backend-context .

# 或交叉编译其他平台
# macOS ARM64 (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o byjyedu-backend-context-darwin-arm64 .

# macOS Intel
GOOS=darwin GOARCH=amd64 go build -o byjyedu-backend-context-darwin-amd64 .

# Linux
GOOS=linux GOARCH=amd64 go build -o byjyedu-backend-context-linux-amd64 .

# Windows
GOOS=windows GOARCH=amd64 go build -o byjyedu-backend-context-windows-amd64.exe .
```

## 验证服务

```bash
# 健康检查
curl http://localhost:3100/health
# {"status":"ok","routes":1020,"vos":1352}

# 手动测试路由列表（MCP JSON-RPC）
curl -X POST http://localhost:3100/message \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_routes","arguments":{"filter":"meeting"}}}'
```

## 故障排查

**Q: 前端连接不上后端的 MCP Server？**

检查以下几点：
1. 后端同事是否已启动服务（`curl http://后端IP:3100/health`）
2. 两台电脑是否在同一局域网
3. macOS 防火墙：系统设置 → 网络 → 防火墙 → 允许传入连接
4. Windows 防火墙：控制面板 → Windows Defender 防火墙 → 允许应用通过防火墙

**Q: 端口被占用了？**

```bash
# 换一个端口
./byjyedu-backend-context --dir /path/to/backend --port 8080
```

**Q: 扫描结果为空或路由数不对？**

```bash
# 确认 --dir 指向的是包含 pom.xml 的项目根目录
# 确认 Controller 文件使用了 @RestController 或 @Controller 注解
```

## 为什么选 Go？

- **单个二进制文件**：编译后发给同事直接用，不需要装 Node.js、Python 或任何运行时
- **零外部依赖**：只用 Go 标准库，没有第三方包
- **跨平台**：一次编译，支持 macOS / Linux / Windows
- **启动快**：秒级启动，毫秒级响应

## 安全说明

- 只读取后端代码文件，**不修改任何文件**
- 只暴露接口路由、参数类型、VO 字段名等结构信息，**不暴露敏感数据**
- 建议仅在**可信局域网**内使用
- 建议代码更新后调用 `refresh_cache`，无需重启服务

## License

MIT
