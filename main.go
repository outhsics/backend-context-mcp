package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	appName    = "backend-context-mcp"
	appVersion = "1.1.0"
)

func main() {
	portFlag := flag.String("port", "", "服务端口，支持 auto")
	host := flag.String("host", "", "监听地址")
	dir := flag.String("dir", "", "后端项目根目录路径")
	configPath := flag.String("config", "", "配置文件路径")
	allowSourceTools := flag.Bool("allow-source-tools", false, "允许 MCP 客户端读取 Service 源码")
	version := flag.Bool("version", false, "打印版本号")
	flag.Parse()

	if *version {
		fmt.Printf("%s %s\n", appName, appVersion)
		return
	}

	if *dir != "" {
		backendRoot = *dir
	} else {
		if wd, err := os.Getwd(); err == nil {
			backendRoot = wd
		}
	}

	if _, err := os.Stat(backendRoot); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "错误: 后端项目目录不存在: %s\n", backendRoot)
		os.Exit(1)
	}

	var err error
	appConfig, err = loadConfig(backendRoot, *configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "配置错误: %v\n", err)
		os.Exit(1)
	}
	if *host != "" {
		appConfig.Server.Host = *host
	}
	if *allowSourceTools {
		appConfig.Security.AllowSourceTools = true
	}
	port, autoPort, err := resolvePort(*portFlag, appConfig.Server.Port)
	if err != nil {
		fmt.Fprintf(os.Stderr, "端口配置错误: %v\n", err)
		os.Exit(1)
	}
	appConfig.Server.Port = port

	scanAll()

	mux := http.NewServeMux()

	// SSE 端点 — MCP 客户端连接
	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(os.Stderr, "[SSE] 新连接来自 %s\n", r.RemoteAddr)

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "不支持 SSE", http.StatusInternalServerError)
			return
		}

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				fmt.Fprintf(w, ": heartbeat\n\n")
				flusher.Flush()
			case <-r.Context().Done():
				fmt.Fprintf(os.Stderr, "[SSE] 连接断开 %s\n", r.RemoteAddr)
				return
			}
		}
	})

	// Messages 端点 — MCP JSON-RPC
	mux.HandleFunc("/message", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(200)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSONRPCError(w, nil, -32700, "Parse error")
			return
		}

		var request map[string]interface{}
		if err := json.Unmarshal(body, &request); err != nil {
			writeJSONRPCError(w, nil, -32700, "Parse error")
			return
		}

		handleMcpRequest(w, request)
	})

	// 健康检查
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":           "ok",
			"name":             appName,
			"version":          appVersion,
			"framework":        appConfig.Framework,
			"routes":           len(routesCache),
			"schemas":          len(vosCache),
			"allowSourceTools": appConfig.Security.AllowSourceTools,
			"backendRoot":      backendRoot,
		})
	})

	listener, err := listenWithOptionalAutoPort(appConfig.Server.Host, appConfig.Server.Port, autoPort)
	if err != nil {
		fmt.Fprintf(os.Stderr, "启动失败: %v\n", err)
		os.Exit(1)
	}
	actualPort := listener.Addr().(*net.TCPAddr).Port
	appConfig.Server.Port = actualPort
	displayHost := appConfig.Server.Host
	if displayHost == "0.0.0.0" || displayHost == "::" {
		displayHost = getLocalIP()
	}

	fmt.Printf("\n%s MCP Server 已启动\n", appName)
	fmt.Printf("   后端代码: %s\n", backendRoot)
	fmt.Printf("   框架: %s\n", appConfig.Framework)
	fmt.Printf("   SSE 端点: http://%s:%d/sse\n", displayHost, actualPort)
	fmt.Printf("   健康检查: http://%s:%d/health\n\n", displayHost, actualPort)
	fmt.Printf("前端 .vscode/mcp.json 配置:\n")
	fmt.Printf(`{
  "mcpServers": {
    "backend-context": {
      "url": "http://%s:%d/sse"
    }
  }
}`, displayHost, actualPort)
	fmt.Println()

	server := &http.Server{Handler: mux}
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "启动失败: %v\n", err)
		os.Exit(1)
	}
}

func resolvePort(flagValue string, configPort int) (int, bool, error) {
	if envPort := os.Getenv("PORT"); flagValue == "" && envPort != "" {
		flagValue = envPort
	}
	if flagValue == "" {
		return configPort, true, nil
	}
	if strings.EqualFold(flagValue, "auto") {
		return configPort, true, nil
	}
	port, err := strconv.Atoi(flagValue)
	if err != nil || port <= 0 || port > 65535 {
		return 0, false, fmt.Errorf("invalid port %q", flagValue)
	}
	return port, false, nil
}

func listenWithOptionalAutoPort(host string, port int, auto bool) (net.Listener, error) {
	if host == "" {
		host = "127.0.0.1"
	}
	if port == 0 {
		port = 3100
	}
	attempts := 1
	if auto {
		attempts = 50
	}
	var lastErr error
	for i := 0; i < attempts; i++ {
		addr := fmt.Sprintf("%s:%d", host, port+i)
		listener, err := net.Listen("tcp", addr)
		if err == nil {
			return listener, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func writeJSONRPCError(w http.ResponseWriter, id interface{}, code int, message string) {
	json.NewEncoder(w).Encode(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}

func handleMcpRequest(w http.ResponseWriter, request map[string]interface{}) {
	id := request["id"]
	method, _ := request["method"].(string)
	params, _ := request["params"].(map[string]interface{})

	switch method {
	case "initialize":
		json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      id,
			"result": map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities": map[string]interface{}{
					"tools": map[string]interface{}{},
				},
				"serverInfo": map[string]interface{}{
					"name":    appName,
					"version": appVersion,
				},
			},
		})

	case "notifications/initialized":
		json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      id,
			"result":  nil,
		})

	case "tools/list":
		json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      id,
			"result": map[string]interface{}{
				"tools": getToolDefinitions(),
			},
		})

	case "tools/call":
		toolName, _ := params["name"].(string)
		args, _ := params["arguments"].(map[string]interface{})
		resultText := callTool(toolName, args)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      id,
			"result": map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": resultText},
				},
			},
		})

	default:
		json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      id,
			"result":  nil,
		})
	}
}

func getToolDefinitions() []map[string]interface{} {
	tools := []map[string]interface{}{
		{
			"name":        "get_project_summary",
			"description": "Return backend project scan summary, framework, API count, schema count, and security settings.",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			"name":        "list_apis",
			"description": "List backend API routes. Supports keyword filtering and markdown/json output.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"filter": map[string]interface{}{
						"type":        "string",
						"description": "Keyword matched against path, summary, tag, module, or HTTP method.",
					},
					"format": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"markdown", "json"},
						"description": "Output format. Defaults to markdown.",
					},
				},
			},
		},
		{
			"name":        "get_api",
			"description": "Get one API detail including request params, response schema, permissions, and source location.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "API path, for example /users/{id}.",
					},
					"format": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"markdown", "json"},
						"description": "Output format. Defaults to markdown.",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			"name":        "list_schemas",
			"description": "List scanned DTO/VO/schema classes. Supports keyword filtering and markdown/json output.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"keyword": map[string]interface{}{
						"type":        "string",
						"description": "Schema class name or field keyword.",
					},
					"format": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"markdown", "json"},
						"description": "Output format. Defaults to markdown.",
					},
				},
			},
		},
		{
			"name":        "get_schema",
			"description": "Get a DTO/VO/schema class with field definitions.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Schema class name.",
					},
					"format": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"markdown", "json"},
						"description": "Output format. Defaults to markdown.",
					},
				},
				"required": []string{"name"},
			},
		},
		{
			"name":        "list_routes",
			"description": "Deprecated alias of list_apis.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"filter": map[string]interface{}{
						"type":        "string",
						"description": "筛选关键词（路径/模块/方法名）",
					},
				},
			},
		},
		{
			"name":        "get_api_detail",
			"description": "Deprecated alias of get_api.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "接口路径，如 /edu-teaching/meeting/page",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			"name":        "search_vo",
			"description": "Deprecated alias of list_schemas.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"keyword": map[string]interface{}{
						"type":        "string",
						"description": "VO 类名或字段名关键词",
					},
				},
				"required": []string{"keyword"},
			},
		},
		{
			"name":        "get_service_logic",
			"description": "Read Service-layer source code. Disabled by default; requires security.allowSourceTools=true.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"className": map[string]interface{}{
						"type":        "string",
						"description": "Service 类名（如 MeetingService 或 MeetingServiceImpl）",
					},
				},
				"required": []string{"className"},
			},
		},
		{
			"name":        "refresh_cache",
			"description": "刷新后端代码缓存。当后端更新代码后调用此工具获取最新信息。",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}
	return tools
}

func callTool(name string, args map[string]interface{}) string {
	switch name {
	case "get_project_summary":
		return jsonText(projectSummary())

	case "list_routes", "list_apis":
		filter := ""
		if v, ok := args["filter"]; ok {
			filter = fmt.Sprintf("%v", v)
		}
		if wantsJSON(args) {
			return jsonText(filterRoutes(filter))
		}
		return formatRouteTable(filter)

	case "get_api_detail", "get_api":
		p, _ := args["path"].(string)
		if wantsJSON(args) {
			route := findRouteByPath(p)
			if route == nil {
				return jsonText(map[string]interface{}{"error": "api not found", "path": p})
			}
			return jsonText(route)
		}
		return formatApiDetail(p)

	case "search_vo", "list_schemas":
		kw, _ := args["keyword"].(string)
		if wantsJSON(args) {
			return jsonText(filterSchemas(kw))
		}
		return formatVoSearch(kw)

	case "get_schema":
		name, _ := args["name"].(string)
		ensureScanned()
		if vo, ok := vosCache[name]; ok {
			if wantsJSON(args) {
				return jsonText(vo)
			}
			return fmt.Sprintf("## %s\n%s\n\nFile: %s", vo.Name, formatVoFields(vo), vo.SourceFile)
		}
		return fmt.Sprintf("未找到 schema: %s", name)

	case "get_service_logic":
		if !appConfig.Security.AllowSourceTools {
			return "源码读取工具默认关闭。请在配置中设置 security.allowSourceTools=true，或启动时添加 --allow-source-tools。"
		}
		cn, _ := args["className"].(string)
		filePath, err := findServiceFile(cn)
		if err != nil {
			// 模糊搜索
			serviceName := strings.ReplaceAll(cn, "Service", "")
			serviceName = strings.ReplaceAll(serviceName, "Impl", "")
			javaFiles, _ := walkJavaFiles(backendRoot)
			var matches []string
			for _, f := range javaFiles {
				if pathHasSegment(f, appConfig.ServicePaths) &&
					strings.Contains(strings.ToLower(filepath.Base(f)), strings.ToLower(serviceName)) {
					rel := strings.TrimPrefix(f, backendRoot)
					matches = append(matches, fmt.Sprintf("  - %s", rel))
				}
			}
			if len(matches) > 0 {
				return fmt.Sprintf("未精确找到 \"%s\"，但发现以下相关文件：\n%s\n\n请用精确类名重试。", cn, strings.Join(matches, "\n"))
			}
			return fmt.Sprintf("未找到 \"%s\" 相关的 Service 文件。", cn)
		}
		content, _ := os.ReadFile(filePath)
		rel := strings.TrimPrefix(filePath, backendRoot)
		return fmt.Sprintf("📁 %s\n\n```java\n%s\n```", rel, string(content))

	case "refresh_cache":
		routesCache = nil
		vosCache = nil
		scanAll()
		return fmt.Sprintf("缓存已刷新！%d 个路由, %d 个 VO。", len(routesCache), len(vosCache))

	default:
		return fmt.Sprintf("未知工具: %s", name)
	}
}

func wantsJSON(args map[string]interface{}) bool {
	format, _ := args["format"].(string)
	return strings.EqualFold(format, "json")
}

func jsonText(v interface{}) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(data)
}

func projectSummary() map[string]interface{} {
	ensureScanned()
	return map[string]interface{}{
		"name":             appName,
		"version":          appVersion,
		"framework":        appConfig.Framework,
		"backendRoot":      backendRoot,
		"apis":             len(routesCache),
		"schemas":          len(vosCache),
		"allowSourceTools": appConfig.Security.AllowSourceTools,
		"controllerPaths":  appConfig.ControllerPaths,
		"dtoPaths":         appConfig.DtoPaths,
		"servicePaths":     appConfig.ServicePaths,
	}
}

func filterRoutes(filter string) []ApiRoute {
	ensureScanned()
	if filter == "" {
		return routesCache
	}
	f := strings.ToLower(filter)
	var filtered []ApiRoute
	for _, r := range routesCache {
		if strings.Contains(strings.ToLower(r.FullPath), f) ||
			strings.Contains(strings.ToLower(r.Summary), f) ||
			strings.Contains(strings.ToLower(r.Tag), f) ||
			strings.Contains(strings.ToLower(r.Module), f) ||
			strings.ToLower(r.Method) == f {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func findRouteByPath(apiPath string) *ApiRoute {
	ensureScanned()
	normalized := apiPath
	if !strings.HasPrefix(normalized, "/") {
		normalized = "/" + normalized
	}
	for i := range routesCache {
		r := &routesCache[i]
		if r.FullPath == normalized || strings.HasSuffix(r.FullPath, normalized) || strings.Contains(r.FullPath, normalized) {
			return r
		}
	}
	return nil
}

func filterSchemas(keyword string) []VoClass {
	ensureScanned()
	kw := strings.ToLower(keyword)
	var matches []VoClass
	for _, vo := range vosCache {
		if kw == "" || strings.Contains(strings.ToLower(vo.Name), kw) {
			matches = append(matches, vo)
			continue
		}
		for _, f := range vo.Fields {
			if strings.Contains(strings.ToLower(f.Name), kw) {
				matches = append(matches, vo)
				break
			}
		}
	}
	return matches
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "本机IP"
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			return ipnet.IP.String()
		}
	}
	return "本机IP"
}
