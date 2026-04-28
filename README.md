# Backend Context MCP

Local MCP server that exposes backend API routes, schemas, permissions, and limited source context to AI coding assistants.

It is designed for teams where frontend developers ask AI tools to integrate with backend APIs. The backend developer runs one local binary, and the AI assistant can query API context through MCP.

## Highlights

- Single Go binary for macOS, Linux, and Windows.
- npm wrapper for `npx backend-context-mcp`.
- Spring MVC source scanning for controllers, DTOs, and services.
- OpenAPI JSON import from `openapi.json`, `swagger.json`, a configured file, or URL.
- Configurable project conventions instead of hard-coded company paths.
- Safe defaults: listens on `127.0.0.1`, auto-avoids occupied ports, source-reading tools are disabled unless explicitly enabled.
- Markdown output for humans and JSON output for reliable AI consumption.

## Quick Start

```bash
npx backend-context-mcp --dir /path/to/backend
```

Or run a downloaded binary:

```bash
./backend-context-mcp-darwin-arm64 --dir /path/to/backend
```

The server prints the MCP URL:

```text
backend-context-mcp MCP Server started
   Backend root: /path/to/backend
   SSE endpoint: http://127.0.0.1:3100/sse
   Health check: http://127.0.0.1:3100/health
```

Add it to a frontend project:

```json
{
  "mcpServers": {
    "backend-context": {
      "url": "http://127.0.0.1:3100/sse"
    }
  }
}
```

For remote teammates, run with an explicit host:

```bash
backend-context-mcp --dir /path/to/backend --host 0.0.0.0 --port 3100
```

Only do this on a trusted network.

## CLI

```bash
backend-context-mcp [options]

Options:
  --dir string              Backend project root. Defaults to current directory.
  --config string           Config file path.
  --host string             Listen host. Defaults to config or 127.0.0.1.
  --port string             Port number or "auto". Defaults to config port with auto fallback.
  --allow-source-tools      Enable Service source-reading tools.
  --version                 Print version.
```

`PORT` is also supported:

```bash
PORT=3200 backend-context-mcp --dir ./backend
```

## Configuration

Create `backend-context.config.json` in the backend root, or pass `--config`.

```json
{
  "framework": "spring",
  "include": ["**/*.java"],
  "excludeDirs": [".git", "target", "build", ".gradle", "node_modules"],
  "controllerPaths": ["controller", "api", "resource", "web"],
  "dtoPaths": ["vo", "dto", "model", "request", "response", "payload"],
  "servicePaths": ["service", "application", "usecase"],
  "dtoMarkers": ["VO", "DTO", "Dto", "Request", "Response", "Payload", "Command", "Query", "Model"],
  "modulePattern": "(?:^|/)(?:[\\w-]+-)?(?:module|service|app|api)-([\\w-]+)(?:/|$)",
  "openapi": {
    "file": "",
    "url": ""
  },
  "security": {
    "allowSourceTools": false
  },
  "server": {
    "host": "127.0.0.1",
    "port": 3100
  }
}
```

See [examples/backend-context.config.json](examples/backend-context.config.json).

## MCP Tools

| Tool | Purpose |
| --- | --- |
| `get_project_summary` | Return scan status, framework, API count, schema count, and security settings. |
| `list_apis` | List API routes. Supports `filter` and `format: "json"`. |
| `get_api` | Return one API detail by path. Supports `format: "json"`. |
| `list_schemas` | Search DTO/schema classes by class or field name. |
| `get_schema` | Return one schema class with fields. |
| `refresh_cache` | Rescan backend files. |
| `get_service_logic` | Read Service source code. Disabled by default. |

Deprecated aliases remain available for compatibility: `list_routes`, `get_api_detail`, and `search_vo`.

## OpenAPI

If your backend already exposes OpenAPI, prefer that integration because it is framework-neutral and more stable than source parsing.

Auto-detected files:

- `openapi.json`
- `swagger.json`
- `api-docs.json`

Explicit file:

```json
{
  "openapi": {
    "file": "./docs/openapi.json"
  }
}
```

Explicit URL:

```json
{
  "openapi": {
    "url": "http://localhost:8080/v3/api-docs"
  }
}
```

## Security

By default, the server:

- listens on `127.0.0.1`;
- exposes API and schema metadata;
- does not expose Service source code.

To enable source-reading tools:

```bash
backend-context-mcp --dir ./backend --allow-source-tools
```

Or:

```json
{
  "security": {
    "allowSourceTools": true
  }
}
```

Do not enable source tools on an untrusted network.

## Windows Support

For detailed Windows installation and usage instructions, see [WINDOWS.md](WINDOWS.md).

Quick start on Windows:
```cmd
backend-context-mcp.exe --dir C:\path\to\backend
```

## Build

**macOS/Linux:**
```bash
go test ./...
bash build.sh
```

**Windows:**
```powershell
# PowerShell
.\build.ps1

# Or double-click build.bat
```

Release assets are written to `dist/`:

- `backend-context-mcp-darwin-arm64`
- `backend-context-mcp-darwin-amd64`
- `backend-context-mcp-linux-amd64`
- `backend-context-mcp-linux-arm64`
- `backend-context-mcp-windows-amd64.exe`

## Publish

GitHub Release:

```bash
git tag v1.1.0
git push origin v1.1.0
```

The GitHub Actions workflow builds and uploads release assets.

npm:

```bash
npm publish --access public
```

The npm package downloads the matching binary from the GitHub release during install.

## License

MIT
