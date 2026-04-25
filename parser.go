package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ─── 类型定义 ───

type ApiRoute struct {
	Module        string      `json:"module"`
	Tag           string      `json:"tag"`
	BasePath      string      `json:"basePath"`
	Method        string      `json:"method"`
	FullPath      string      `json:"fullPath"`
	MethodName    string      `json:"methodName"`
	Summary       string      `json:"summary"`
	Permission    string      `json:"permission"`
	RequestParams []ParamInfo `json:"requestParams"`
	ReturnType    string      `json:"returnType"`
	SourceFile    string      `json:"sourceFile"`
}

type ParamInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
	ParamSource string `json:"paramSource"` // body, query, param, path
	VoClass     string `json:"voClass,omitempty"`
}

type VoField struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Required    bool     `json:"required"`
	Annotations []string `json:"annotations"`
}

type VoClass struct {
	Name        string    `json:"name"`
	PackageName string    `json:"packageName"`
	Description string    `json:"description"`
	Fields      []VoField `json:"fields"`
	ParentClass string    `json:"parentClass,omitempty"`
	SourceFile  string    `json:"sourceFile"`
}

// ─── 全局缓存 ───

var (
	routesCache []ApiRoute
	vosCache    map[string]VoClass
	backendRoot string
)

var skipDirs = map[string]bool{
	".git": true, "node_modules": true, ".idea": true, "target": true,
	".mvn": true, "dist": true, "build": true, ".vscode": true,
	".mcp-backend-context": true,
}

// ─── 文件遍历 ───

func walkJavaFiles(dir string) ([]string, error) {
	var results []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return results, err
	}
	for _, entry := range entries {
		if skipDirs[entry.Name()] {
			continue
		}
		fullPath := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			sub, err := walkJavaFiles(fullPath)
			if err != nil {
				continue
			}
			results = append(results, sub...)
		} else if strings.HasSuffix(entry.Name(), ".java") {
			results = append(results, fullPath)
		}
	}
	return results, nil
}

func getModuleFromPath(filePath string) string {
	re := regexp.MustCompile(`byjedu-module-([\w-]+)`)
	matches := re.FindAllStringSubmatch(filePath, -1)
	if len(matches) == 0 {
		return "unknown"
	}
	parts := make([]string, len(matches))
	for i, m := range matches {
		parts[i] = m[1]
	}
	return strings.Join(parts, "/")
}

// ─── Controller 解析 ───

func parseControllerFile(filePath string) ([]ApiRoute, string, string) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, "", ""
	}
	text := string(content)

	if !strings.Contains(text, "@RestController") && !strings.Contains(text, "@Controller") {
		return nil, "", ""
	}

	module := getModuleFromPath(filePath)

	// @Tag
	tagRe := regexp.MustCompile(`@Tag\s*\(\s*name\s*=\s*"([^"]+)"`)
	tagMatch := tagRe.FindStringSubmatch(text)
	tag := "Unknown"
	if len(tagMatch) > 1 {
		tag = tagMatch[1]
	}

	// @RequestMapping prefix
	baseRe := regexp.MustCompile(`@RequestMapping\s*\(\s*"([^"]+)"`)
	baseMatch := baseRe.FindStringSubmatch(text)
	basePath := ""
	if len(baseMatch) > 1 {
		basePath = baseMatch[1]
	}

	// 更稳健的方式：先按方法签名定位，再往前找注解
	sigRe := regexp.MustCompile(`public\s+([\w<>,\s?.\[\]]+?)\s+(\w+)\s*\(([^)]*)\)\s*\{`)
	sigMatches := sigRe.FindAllStringSubmatchIndex(text, -1)

	var routes []ApiRoute

	for _, loc := range sigMatches {
		javaMethodName := extractGroup(text, loc, 2)
		if javaMethodName == "" {
			continue
		}

		returnType := strings.TrimSpace(extractGroup(text, loc, 1))
		paramsBlock := extractGroup(text, loc, 3)

		// 往前找注解区域（从上一个 } 或 class { 开始到当前位置）
		methodStart := loc[0]
		annotationStart := findAnnotationStart(text, methodStart)
		annotationBlock := text[annotationStart:methodStart]

		// 找 HTTP mapping
		mapRe := regexp.MustCompile(`@(Get|Post|Put|Delete|Patch)Mapping\s*(?:\(\s*(?:"([^"]*)")?\s*\))?\s*`)
		mapMatch := mapRe.FindStringSubmatch(annotationBlock)
		if len(mapMatch) < 2 {
			continue
		}
		httpMethod := strings.ToUpper(mapMatch[1])
		methodPath := ""
		if len(mapMatch) > 2 {
			methodPath = mapMatch[2]
		}

		// @Operation
		opRe := regexp.MustCompile(`@Operation\s*\([^)]*summary\s*=\s*"([^"]*)"`)
		opMatch := opRe.FindStringSubmatch(annotationBlock)
		summary := javaMethodName
		if len(opMatch) > 1 {
			summary = opMatch[1]
		}

		// @PreAuthorize
		permRe := regexp.MustCompile(`@PreAuthorize\s*\(\s*"([^"]+)"`)
		permMatch := permRe.FindStringSubmatch(annotationBlock)
		permission := ""
		if len(permMatch) > 1 {
			permission = permMatch[1]
		}

		params := parseMethodParams(paramsBlock)
		innerReturn := extractGenericType(returnType)

		relPath := strings.TrimPrefix(filePath, backendRoot)

		routes = append(routes, ApiRoute{
			Module:        module,
			Tag:           tag,
			BasePath:      basePath,
			Method:        httpMethod,
			FullPath:      basePath + methodPath,
			MethodName:    javaMethodName,
			Summary:       summary,
			Permission:    permission,
			RequestParams: params,
			ReturnType:    innerReturn,
			SourceFile:    relPath,
		})
	}

	return routes, tag, basePath
}

func extractGroup(text string, loc []int, group int) string {
	start := loc[group*2]
	end := loc[group*2+1]
	if start < 0 || end < 0 || start > len(text) || end > len(text) {
		return ""
	}
	return text[start:end]
}

func findAnnotationStart(text string, methodStart int) int {
	// 从方法签名往前找注解区域
	// 1. 跳过空白
	// 2. 遇到 } 说明是上一个方法的结束，跳过它
	// 3. 跳过空白
	// 4. 往前收集 @ 开头的注解行
	// 5. 遇到非注解行就停

	pos := methodStart - 1

	// 跳过空白
	for pos > 0 && isWhitespace(text[pos]) {
		pos--
	}

	// 跳过上一个方法的 }
	if pos > 0 && text[pos] == '}' {
		pos--
		for pos > 0 && isWhitespace(text[pos]) {
			pos--
		}
	}

	// 往前收集 @ 注解行
	for pos > 0 {
		lineStart := pos
		for lineStart > 0 && text[lineStart-1] != '\n' {
			lineStart--
		}
		line := strings.TrimSpace(text[lineStart : pos+1])
		if !strings.HasPrefix(line, "@") {
			break
		}
		pos = lineStart - 1
	}

	if pos < 0 {
		pos = 0
	}
	return pos
}

func isWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'
}

func extractGenericType(raw string) string {
	re := regexp.MustCompile(`<([^>]+)>`)
	matches := re.FindAllStringSubmatch(raw, -1)
	if len(matches) == 0 {
		return strings.TrimSpace(raw)
	}
	last := matches[len(matches)-1]
	inner := strings.TrimSpace(last[1])
	// 如果还有嵌套泛型，取最内层
	if strings.Contains(inner, "<") {
		return extractGenericType(inner)
	}
	return inner
}

func parseMethodParams(paramsBlock string) []ParamInfo {
	paramsBlock = strings.TrimSpace(paramsBlock)
	if paramsBlock == "" {
		return nil
	}

	var params []ParamInfo

	// 简单按逗号分割，但注意泛型中的逗号
	parts := splitParams(paramsBlock)

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		param := ParamInfo{ParamSource: "query"}

		if strings.Contains(part, "@RequestBody") {
			param.ParamSource = "body"
			param.Required = true
		}

		if strings.Contains(part, "@RequestParam") {
			param.ParamSource = "param"
			param.Required = true // 默认 required
			reqRe := regexp.MustCompile(`required\s*=\s*(true|false)`)
			reqMatch := reqRe.FindStringSubmatch(part)
			if len(reqMatch) > 1 {
				param.Required = reqMatch[1] == "true"
			}
			nameRe := regexp.MustCompile(`@RequestParam\s*\(\s*(?:"([^"]+)"|value\s*=\s*"([^"]+)")`)
			nameMatch := nameRe.FindStringSubmatch(part)
			if len(nameMatch) > 1 {
				if nameMatch[1] != "" {
					param.Name = nameMatch[1]
				} else if len(nameMatch) > 2 {
					param.Name = nameMatch[2]
				}
			}
		}

		if strings.Contains(part, "@PathVariable") {
			param.ParamSource = "path"
			param.Required = true
		}

		// @Parameter description
		descRe := regexp.MustCompile(`@Parameter[^)]*description\s*=\s*"([^"]*)"`)
		descMatch := descRe.FindStringSubmatch(part)
		if len(descMatch) > 1 {
			param.Description = descMatch[1]
		}

		// Type + Name (最后一个)
		typeRe := regexp.MustCompile(`(\w+(?:<[^>]+>)?)\s+(\w+)\s*$`)
		typeMatch := typeRe.FindStringSubmatch(part)
		if len(typeMatch) > 2 {
			param.Type = typeMatch[1]
			if param.Name == "" {
				param.Name = typeMatch[2]
			}
			if strings.Contains(param.Type, "VO") || strings.Contains(param.Type, "Dto") || strings.Contains(param.Type, "DTO") {
				re := regexp.MustCompile(`(\w+)`)
				param.VoClass = re.FindString(param.Type)
			}
		}

		if param.Name != "" {
			params = append(params, param)
		}
	}

	return params
}

func splitParams(s string) []string {
	var parts []string
	depth := 0
	current := strings.Builder{}
	for _, ch := range s {
		if ch == '<' || ch == '(' {
			depth++
		} else if ch == '>' || ch == ')' {
			depth--
		}
		if ch == ',' && depth == 0 {
			parts = append(parts, current.String())
			current.Reset()
		} else {
			current.WriteRune(ch)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

// ─── VO 解析 ───

func parseVoFile(filePath string) *VoClass {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}
	text := string(content)

	className := strings.TrimSuffix(filepath.Base(filePath), ".java")
	// 宽松匹配：包含 VO / Dto / DTO / Base 的文件
	if !strings.Contains(className, "VO") && !strings.Contains(className, "Vo") &&
		!strings.Contains(className, "Dto") && !strings.Contains(className, "DTO") &&
		!strings.Contains(className, "Base") {
		return nil
	}

	// package
	pkgRe := regexp.MustCompile(`package\s+([\w.]+);`)
	pkgMatch := pkgRe.FindStringSubmatch(text)
	pkgName := ""
	if len(pkgMatch) > 1 {
		pkgName = pkgMatch[1]
	}

	// @Schema
	schemaRe := regexp.MustCompile(`@Schema\s*\([^)]*description\s*=\s*"([^"]*)"`)
	schemaMatch := schemaRe.FindStringSubmatch(text)
	desc := className
	if len(schemaMatch) > 1 {
		desc = schemaMatch[1]
	}

	// extends
	extRe := regexp.MustCompile(`extends\s+(\w+)`)
	extMatch := extRe.FindStringSubmatch(text)
	parentClass := ""
	if len(extMatch) > 1 {
		parentClass = extMatch[1]
	}

	// 字段
	var fields []VoField
	fieldRe := regexp.MustCompile(`(?:@Schema\s*\([^)]*\)\s*)?(?:private|protected|public)\s+(\w+(?:<[^>]+>)?)\s+(\w+)\s*;`)
	fieldMatches := fieldRe.FindAllStringSubmatch(text, -1)

	for _, fm := range fieldMatches {
		if len(fm) < 3 {
			continue
		}
		fieldType := fm[1]
		fieldName := fm[2]

		// 找字段前面的注解区域
		fieldIdx := strings.Index(text, fm[0])
		beforeField := ""
		if fieldIdx > 200 {
			beforeField = text[fieldIdx-200 : fieldIdx]
		} else if fieldIdx > 0 {
			beforeField = text[:fieldIdx]
		}

		var fieldDesc string
		fdRe := regexp.MustCompile(`@Schema\s*\([^)]*description\s*=\s*"([^"]*)"`)
		fdMatch := fdRe.FindStringSubmatch(beforeField)
		if len(fdMatch) > 1 {
			fieldDesc = fdMatch[1]
		}

		required := strings.Contains(beforeField, "required = true")

		fields = append(fields, VoField{
			Name:        fieldName,
			Type:        fieldType,
			Description: fieldDesc,
			Required:    required,
		})
	}

	relPath := strings.TrimPrefix(filePath, backendRoot)

	return &VoClass{
		Name:        className,
		PackageName: pkgName,
		Description: desc,
		Fields:      fields,
		ParentClass: parentClass,
		SourceFile:  relPath,
	}
}

// ─── 全量扫描 ───

func scanAll() {
	javaFiles, err := walkJavaFiles(backendRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "扫描失败: %v\n", err)
		return
	}

	routesCache = nil
	vosCache = make(map[string]VoClass)

	for _, file := range javaFiles {
		// Controller
		if strings.Contains(file, "controller") || strings.Contains(file, "Controller") {
			routes, _, _ := parseControllerFile(file)
			if len(routes) > 0 {
				routesCache = append(routesCache, routes...)
			}
		}

		// VO
		if strings.Contains(file, string(filepath.Separator)+"vo"+string(filepath.Separator)) {
			vo := parseVoFile(file)
			if vo != nil {
				vosCache[vo.Name] = *vo
			}
		}
	}

	fmt.Fprintf(os.Stderr, "扫描完成: %d 个路由, %d 个 VO\n", len(routesCache), len(vosCache))
}

func ensureScanned() {
	if routesCache == nil {
		scanAll()
	}
}

func findServiceFile(className string) (string, error) {
	javaFiles, err := walkJavaFiles(backendRoot)
	if err != nil {
		return "", err
	}
	for _, file := range javaFiles {
		if strings.Contains(file, string(filepath.Separator)+"service"+string(filepath.Separator)) {
			base := filepath.Base(file)
			if base == className+".java" {
				return file, nil
			}
		}
	}
	return "", fmt.Errorf("未找到 %s", className)
}

// ─── 格式化输出 ───

func formatRouteTable(filter string) string {
	ensureScanned()
	var filtered []ApiRoute
	if filter != "" {
		f := strings.ToLower(filter)
		for _, r := range routesCache {
			if strings.Contains(strings.ToLower(r.FullPath), f) ||
				strings.Contains(strings.ToLower(r.Summary), f) ||
				strings.Contains(strings.ToLower(r.Tag), f) ||
				strings.ToLower(r.Method) == f {
				filtered = append(filtered, r)
			}
		}
	} else {
		filtered = routesCache
	}

	if len(filtered) == 0 {
		return "没有找到匹配的路由。"
	}

	var sb strings.Builder
	currentTag := ""

	methodBadge := map[string]string{
		"GET": "[GET]   ", "POST": "[POST]  ", "PUT": "[PUT]   ",
		"DELETE": "[DELETE]", "PATCH": "[PATCH] ",
	}

	for _, r := range filtered {
		if r.Tag != currentTag {
			currentTag = r.Tag
			sb.WriteString(fmt.Sprintf("\n## %s\n", currentTag))
		}
		badge := methodBadge[r.Method]
		if badge == "" {
			badge = fmt.Sprintf("[%s]", r.Method)
		}
		perm := "🌐 无权限限制"
		if r.Permission != "" {
			perm = fmt.Sprintf("🔒 %s", r.Permission)
		}
		sb.WriteString(fmt.Sprintf("  %s %s\n", badge, r.FullPath))
		sb.WriteString(fmt.Sprintf("    %s | %s\n", r.Summary, perm))
		if len(r.RequestParams) > 0 {
			var paramDescs []string
			for _, p := range r.RequestParams {
				req := ""
				if p.Required {
					req = ", 必填"
				}
				paramDescs = append(paramDescs, fmt.Sprintf("%s: %s (%s%s)", p.Name, p.Type, p.ParamSource, req))
			}
			sb.WriteString(fmt.Sprintf("    参数: %s\n", strings.Join(paramDescs, ", ")))
		}
		sb.WriteString(fmt.Sprintf("    返回: %s\n\n", r.ReturnType))
	}

	sb.WriteString(fmt.Sprintf("\n共 %d 个接口", len(filtered)))
	if filter != "" {
		sb.WriteString(fmt.Sprintf(" (筛选: \"%s\")", filter))
	}
	sb.WriteString(fmt.Sprintf(" (总计 %d 个)", len(routesCache)))

	return sb.String()
}

func formatApiDetail(apiPath string) string {
	ensureScanned()

	normalized := apiPath
	if !strings.HasPrefix(normalized, "/") {
		normalized = "/" + normalized
	}

	var route *ApiRoute
	for i := range routesCache {
		r := &routesCache[i]
		if r.FullPath == normalized || strings.HasSuffix(r.FullPath, normalized) || strings.Contains(r.FullPath, normalized) {
			route = r
			break
		}
	}

	if route == nil {
		keyword := strings.TrimPrefix(normalized, "/")
		var matches []string
		for _, r := range routesCache {
			if strings.Contains(r.FullPath, keyword) || strings.Contains(r.Summary, keyword) {
				matches = append(matches, fmt.Sprintf("  %s %s — %s", r.Method, r.FullPath, r.Summary))
			}
		}
		if len(matches) > 0 {
			return fmt.Sprintf("未精确匹配 \"%s\"，但找到以下相关接口：\n\n%s\n\n请用精确路径重试。", apiPath, strings.Join(matches, "\n"))
		}
		return fmt.Sprintf("未找到匹配 \"%s\" 的接口。用 list_routes 搜索一下？", apiPath)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\n\n", route.Summary))
	sb.WriteString("| 属性 | 值 |\n|---|---|\n")
	sb.WriteString(fmt.Sprintf("| 模块 | %s |\n", route.Module))
	sb.WriteString(fmt.Sprintf("| 路径 | %s %s |\n", route.Method, route.FullPath))
	sb.WriteString(fmt.Sprintf("| 权限 | %s |\n", route.Permission))
	if route.Permission == "" {
		sb.WriteString("| 权限 | 无 |\n")
	}
	sb.WriteString(fmt.Sprintf("| 源文件 | %s |\n", route.SourceFile))

	sb.WriteString("\n## 请求参数\n\n")
	if len(route.RequestParams) == 0 {
		sb.WriteString("无参数\n")
	} else {
		sb.WriteString("| 参数名 | 类型 | 来源 | 必填 | 说明 |\n|---|---|---|---|---|\n")
		srcMap := map[string]string{"body": "Request Body", "query": "Query String", "param": "URL Param", "path": "Path Variable"}
		for _, p := range route.RequestParams {
			src := srcMap[p.ParamSource]
			req := "否"
			if p.Required {
				req = "是"
			}
			desc := "-"
			if p.Description != "" {
				desc = p.Description
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n", p.Name, p.Type, src, req, desc))
		}

		// 展开 VO
		for _, p := range route.RequestParams {
			if p.VoClass != "" {
				if vo, ok := vosCache[p.VoClass]; ok {
					sb.WriteString(fmt.Sprintf("\n### %s 字段\n\n", p.VoClass))
					sb.WriteString(formatVoFields(vo))
				}
			}
		}
	}

	sb.WriteString("\n## 返回类型\n\n")
	sb.WriteString(fmt.Sprintf("`%s`\n", route.ReturnType))
	if vo, ok := vosCache[route.ReturnType]; ok {
		sb.WriteString(fmt.Sprintf("\n### %s 字段\n\n", route.ReturnType))
		sb.WriteString(formatVoFields(vo))
	}

	return sb.String()
}

func formatVoFields(vo VoClass) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("> %s\n\n", vo.Description))

	if vo.ParentClass != "" {
		if parent, ok := vosCache[vo.ParentClass]; ok {
			sb.WriteString(fmt.Sprintf("> 继承自 %s，父类字段：\n\n", vo.ParentClass))
			sb.WriteString("| 字段名 | 类型 | 必填 | 说明 |\n|---|---|---|---|\n")
			for _, f := range parent.Fields {
				req := "否"
				if f.Required {
					req = "是"
				}
				desc := "-"
				if f.Description != "" {
					desc = f.Description
				}
				sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", f.Name, f.Type, req, desc))
			}
			sb.WriteString("\n")
		} else {
			sb.WriteString(fmt.Sprintf("> 继承自 `%s`（未扫描到）\n\n", vo.ParentClass))
		}
	}

	sb.WriteString("| 字段名 | 类型 | 必填 | 说明 |\n|---|---|---|---|\n")
	for _, f := range vo.Fields {
		req := "否"
		if f.Required {
			req = "是"
		}
		desc := "-"
		if f.Description != "" {
			desc = f.Description
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", f.Name, f.Type, req, desc))
	}

	return sb.String()
}

func formatVoSearch(keyword string) string {
	ensureScanned()
	kw := strings.ToLower(keyword)
	var matches []VoClass

	for _, vo := range vosCache {
		if strings.Contains(strings.ToLower(vo.Name), kw) {
			matches = append(matches, vo)
		} else {
			for _, f := range vo.Fields {
				if strings.Contains(strings.ToLower(f.Name), kw) {
					matches = append(matches, vo)
					break
				}
			}
		}
	}

	if len(matches) == 0 {
		var names []string
		for name := range vosCache {
			names = append(names, name)
		}
		return fmt.Sprintf("未找到匹配 \"%s\" 的 VO/DTO。已有的 VO：\n%s", keyword, strings.Join(names, ", "))
	}

	var parts []string
	for _, vo := range matches {
		parts = append(parts, fmt.Sprintf("## %s\n%s\n\n📁 %s", vo.Name, formatVoFields(vo), vo.SourceFile))
	}
	return strings.Join(parts, "\n---\n")
}
