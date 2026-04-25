package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func loadOpenAPIContext() ([]ApiRoute, map[string]VoClass, error) {
	data, source, err := readOpenAPIData()
	if err != nil || len(data) == 0 {
		return nil, nil, err
	}

	var doc map[string]interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, nil, fmt.Errorf("parse OpenAPI JSON %s: %w", source, err)
	}

	schemas := parseOpenAPISchemas(doc, source)
	routes := parseOpenAPIRoutes(doc, source)
	return routes, schemas, nil
}

func readOpenAPIData() ([]byte, string, error) {
	if appConfig.OpenAPI.URL != "" {
		resp, err := http.Get(appConfig.OpenAPI.URL)
		if err != nil {
			return nil, appConfig.OpenAPI.URL, err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, appConfig.OpenAPI.URL, fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		data, err := io.ReadAll(resp.Body)
		return data, appConfig.OpenAPI.URL, err
	}

	candidates := []string{}
	if appConfig.OpenAPI.File != "" {
		candidates = append(candidates, appConfig.OpenAPI.File)
	} else {
		candidates = append(candidates, "openapi.json", "swagger.json", "api-docs.json")
	}

	for _, candidate := range candidates {
		path := candidate
		if !filepath.IsAbs(path) {
			path = filepath.Join(backendRoot, path)
		}
		data, err := os.ReadFile(path)
		if err == nil {
			return data, path, nil
		}
	}
	return nil, "", nil
}

func parseOpenAPIRoutes(doc map[string]interface{}, source string) []ApiRoute {
	paths, _ := doc["paths"].(map[string]interface{})
	var routes []ApiRoute
	for path, rawPathItem := range paths {
		pathItem, _ := rawPathItem.(map[string]interface{})
		for method, rawOperation := range pathItem {
			httpMethod := strings.ToUpper(method)
			if !isHTTPMethod(httpMethod) {
				continue
			}
			operation, _ := rawOperation.(map[string]interface{})
			route := ApiRoute{
				Module:     "openapi",
				Tag:        firstOpenAPITag(operation),
				Method:     httpMethod,
				FullPath:   path,
				MethodName: stringValue(operation["operationId"]),
				Summary:    firstNonEmpty(stringValue(operation["summary"]), stringValue(operation["description"]), stringValue(operation["operationId"])),
				SourceFile: "openapi:" + source,
			}
			if route.Summary == "" {
				route.Summary = httpMethod + " " + path
			}
			route.RequestParams = parseOpenAPIParams(operation)
			route.ReturnType = parseOpenAPIResponseType(operation)
			routes = append(routes, route)
		}
	}
	return routes
}

func parseOpenAPISchemas(doc map[string]interface{}, source string) map[string]VoClass {
	result := make(map[string]VoClass)
	components, _ := doc["components"].(map[string]interface{})
	schemas, _ := components["schemas"].(map[string]interface{})
	for name, rawSchema := range schemas {
		schema, _ := rawSchema.(map[string]interface{})
		vo := VoClass{
			Name:        name,
			Description: firstNonEmpty(stringValue(schema["description"]), stringValue(schema["title"]), name),
			SourceFile:  "openapi:" + source,
		}
		required := map[string]bool{}
		for _, item := range arrayValue(schema["required"]) {
			required[stringValue(item)] = true
		}
		properties, _ := schema["properties"].(map[string]interface{})
		for fieldName, rawField := range properties {
			field, _ := rawField.(map[string]interface{})
			vo.Fields = append(vo.Fields, VoField{
				Name:        fieldName,
				Type:        openAPITypeName(field),
				Description: stringValue(field["description"]),
				Required:    required[fieldName],
			})
		}
		result[name] = vo
	}
	return result
}

func parseOpenAPIParams(operation map[string]interface{}) []ParamInfo {
	var params []ParamInfo
	for _, rawParam := range arrayValue(operation["parameters"]) {
		param, _ := rawParam.(map[string]interface{})
		schema, _ := param["schema"].(map[string]interface{})
		params = append(params, ParamInfo{
			Name:        stringValue(param["name"]),
			Type:        openAPITypeName(schema),
			Required:    boolValue(param["required"]),
			Description: stringValue(param["description"]),
			ParamSource: firstNonEmpty(stringValue(param["in"]), "query"),
		})
	}
	if body, ok := operation["requestBody"].(map[string]interface{}); ok {
		params = append(params, ParamInfo{
			Name:        "body",
			Type:        openAPIContentType(body),
			Required:    boolValue(body["required"]),
			Description: stringValue(body["description"]),
			ParamSource: "body",
			VoClass:     openAPIContentType(body),
		})
	}
	return params
}

func parseOpenAPIResponseType(operation map[string]interface{}) string {
	responses, _ := operation["responses"].(map[string]interface{})
	for _, code := range []string{"200", "201", "default"} {
		if response, ok := responses[code].(map[string]interface{}); ok {
			return openAPIContentType(response)
		}
	}
	return ""
}

func openAPIContentType(container map[string]interface{}) string {
	content, _ := container["content"].(map[string]interface{})
	for _, rawMedia := range content {
		media, _ := rawMedia.(map[string]interface{})
		schema, _ := media["schema"].(map[string]interface{})
		return openAPITypeName(schema)
	}
	return ""
}

func openAPITypeName(schema map[string]interface{}) string {
	if ref := stringValue(schema["$ref"]); ref != "" {
		return strings.TrimPrefix(ref[strings.LastIndex(ref, "/")+1:], "#")
	}
	if items, ok := schema["items"].(map[string]interface{}); ok {
		return "[]" + openAPITypeName(items)
	}
	if typ := stringValue(schema["type"]); typ != "" {
		return typ
	}
	return "object"
}

func firstOpenAPITag(operation map[string]interface{}) string {
	tags := arrayValue(operation["tags"])
	if len(tags) > 0 {
		return stringValue(tags[0])
	}
	return "OpenAPI"
}

func isHTTPMethod(method string) bool {
	switch method {
	case "GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD", "TRACE":
		return true
	default:
		return false
	}
}

func stringValue(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func boolValue(v interface{}) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

func arrayValue(v interface{}) []interface{} {
	if a, ok := v.([]interface{}); ok {
		return a
	}
	return nil
}
