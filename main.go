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
	"strings"
	"time"
)

func main() {
	port := flag.Int("port", 3000, "服务端口")
	host := flag.String("host", "0.0.0.0", "监听地址")
	dir := flag.String("dir", "", "后端项目根目录路径")
	flag.Parse()

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

	scanAll()

	// SSE 端点 — MCP 客户端连接
	http.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
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
	http.HandleFunc("/message", func(w http.ResponseWriter, r *http.Request) {
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
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"routes": len(routesCache),
			"vos":    len(vosCache),
		})
	})

	addr := fmt.Sprintf("%s:%d", *host, *port)
	fmt.Printf("\n🚀 byjyedu-backend-context MCP Server 已启动\n")
	fmt.Printf("   后端代码: %s\n", backendRoot)
	fmt.Printf("   SSE 端点: http://%s:%d/sse\n", getLocalIP(), *port)
	fmt.Printf("   健康检查: http://%s:%d/health\n\n", getLocalIP(), *port)
	fmt.Printf("前端 .vscode/mcp.json 配置:\n")
	fmt.Printf(`{
  "mcpServers": {
    "backend-context": {
      "url": "http://%s:%d/sse"
    }
  }
}`, getLocalIP(), *port)
	fmt.Println("\n")

	if err := http.ListenAndServe(addr, nil); err != nil {
		fmt.Fprintf(os.Stderr, "启动失败: %v\n", err)
		os.Exit(1)
	}
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
					"name":    "byjyedu-backend-context",
					"version": "1.0.0",
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
	return []map[string]interface{}{
		{
			"name":        "list_routes",
			"description": "列出后端所有 API 路由，可按关键词筛选。返回路由表：路径、HTTP方法、说明、权限、参数类型、返回类型。",
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
			"description": "获取指定 API 接口的完整上下文：请求参数、返回值、VO 字段定义。前端联调核心工具。",
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
			"description": "搜索后端 VO/DTO 类，查看字段定义。可按类名或字段名搜索。",
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
			"description": "读取后端 Service 层代码，理解业务逻辑。适合排查接口行为问题。",
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
}

func callTool(name string, args map[string]interface{}) string {
	switch name {
	case "list_routes":
		filter := ""
		if v, ok := args["filter"]; ok {
			filter = fmt.Sprintf("%v", v)
		}
		return formatRouteTable(filter)

	case "get_api_detail":
		p, _ := args["path"].(string)
		return formatApiDetail(p)

	case "search_vo":
		kw, _ := args["keyword"].(string)
		return formatVoSearch(kw)

	case "get_service_logic":
		cn, _ := args["className"].(string)
		filePath, err := findServiceFile(cn)
		if err != nil {
			// 模糊搜索
			serviceName := strings.ReplaceAll(cn, "Service", "")
			serviceName = strings.ReplaceAll(serviceName, "Impl", "")
			javaFiles, _ := walkJavaFiles(backendRoot)
			var matches []string
			for _, f := range javaFiles {
				if strings.Contains(f, string(os.PathSeparator)+"service"+string(os.PathSeparator)) &&
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
