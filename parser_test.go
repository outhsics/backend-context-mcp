package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanSpringProject(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "user-service", "src", "main", "java", "com", "example", "controller", "UserController.java"), `
package com.example.controller;

import org.springframework.web.bind.annotation.*;

@RestController
@RequestMapping("/users")
class UserController {
  @GetMapping("/{id}")
  public CommonResult<UserDTO> get(@PathVariable Long id) {
    return null;
  }
}
`)
	mustWrite(t, filepath.Join(root, "user-service", "src", "main", "java", "com", "example", "dto", "UserDTO.java"), `
package com.example.dto;

class UserDTO {
  private Long id;
  private String name;
}
`)

	backendRoot = root
	appConfig = defaultConfig()
	scanAll()

	if len(routesCache) != 1 {
		t.Fatalf("routes = %d, want 1", len(routesCache))
	}
	if routesCache[0].FullPath != "/users/{id}" {
		t.Fatalf("path = %q, want /users/{id}", routesCache[0].FullPath)
	}
	if _, ok := vosCache["UserDTO"]; !ok {
		t.Fatal("UserDTO schema was not scanned")
	}
}

func TestScanOpenAPIJSON(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "openapi.json"), `{
  "openapi": "3.0.0",
  "paths": {
    "/users/{id}": {
      "get": {
        "tags": ["Users"],
        "operationId": "getUser",
        "summary": "Get user",
        "parameters": [
          {"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}}
        ],
        "responses": {
          "200": {
            "description": "OK",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/User"}
              }
            }
          }
        }
      }
    }
  },
  "components": {
    "schemas": {
      "User": {
        "type": "object",
        "required": ["id"],
        "properties": {
          "id": {"type": "integer", "description": "User id"},
          "name": {"type": "string"}
        }
      }
    }
  }
}`)

	backendRoot = root
	appConfig = defaultConfig()
	scanAll()

	if len(routesCache) != 1 {
		t.Fatalf("routes = %d, want 1", len(routesCache))
	}
	if routesCache[0].ReturnType != "User" {
		t.Fatalf("return type = %q, want User", routesCache[0].ReturnType)
	}
	if schema := vosCache["User"]; len(schema.Fields) != 2 {
		t.Fatalf("schema fields = %d, want 2", len(schema.Fields))
	}
}

func mustWrite(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
