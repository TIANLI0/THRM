package coreapp

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/TIANLI0/THRM/internal/ipc"
)

// ipcRequestTypesWithoutCoreHandler 列出有意不进分发表的请求类型，每一项都要写明原因。
var ipcRequestTypesWithoutCoreHandler = map[ipc.RequestType]string{
	// 订阅状态属于连接本身，由 ipc.Server.handleSubscription 直接维护，不经过业务 handler。
	ipc.ReqSubscribeEvents:   "由 ipc.Server.handleSubscription 处理",
	ipc.ReqUnsubscribeEvents: "由 ipc.Server.handleSubscription 处理",
}

// declaredIPCRequestTypes 从 internal/ipc 的源码里读出全部 RequestType 常量（常量名 -> 值），
// 这样新增请求类型时不需要再手工维护一份清单。
func declaredIPCRequestTypes(t *testing.T) map[string]ipc.RequestType {
	t.Helper()

	const ipcDir = "../ipc"
	entries, err := os.ReadDir(ipcDir)
	if err != nil {
		t.Fatalf("读取 internal/ipc 目录失败: %v", err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(ipcDir, name), nil, 0)
		if err != nil {
			t.Fatalf("解析 internal/ipc/%s 失败: %v", name, err)
		}
		files = append(files, file)
	}

	declared := make(map[string]ipc.RequestType)
	for _, file := range files {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				vs := spec.(*ast.ValueSpec)
				ident, ok := vs.Type.(*ast.Ident)
				if !ok || ident.Name != "RequestType" {
					continue
				}
				for i, name := range vs.Names {
					if i >= len(vs.Values) {
						t.Fatalf("RequestType 常量 %s 没有显式的值", name.Name)
					}
					lit, ok := vs.Values[i].(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						t.Fatalf("RequestType 常量 %s 的值不是字符串字面量", name.Name)
					}
					value, err := strconv.Unquote(lit.Value)
					if err != nil {
						t.Fatalf("RequestType 常量 %s 的值无法解析: %v", name.Name, err)
					}
					declared[name.Name] = ipc.RequestType(value)
				}
			}
		}
	}
	return declared
}

// TestEveryIPCRequestTypeHasHandler 保证 internal/ipc 里定义的每个请求类型要么在分发表里
// 有处理函数，要么在豁免清单里写明了原因，新增的请求类型不会悄悄落到"未知的请求类型"。
func TestEveryIPCRequestTypeHasHandler(t *testing.T) {
	declared := declaredIPCRequestTypes(t)
	if len(declared) == 0 {
		t.Fatal("没有从 internal/ipc 解析出任何 RequestType 常量")
	}

	known := make(map[ipc.RequestType]bool, len(declared))
	for name, reqType := range declared {
		known[reqType] = true
		handler, registered := ipcRequestHandlers[reqType]
		_, exempt := ipcRequestTypesWithoutCoreHandler[reqType]
		switch {
		case registered && exempt:
			t.Errorf("ipc.%s (%q) 既注册了处理函数又在豁免清单里", name, reqType)
		case registered && handler == nil:
			t.Errorf("ipc.%s (%q) 注册的处理函数是 nil", name, reqType)
		case !registered && !exempt:
			t.Errorf("ipc.%s (%q) 没有注册处理函数，也不在豁免清单里", name, reqType)
		}
	}

	for reqType := range ipcRequestHandlers {
		if !known[reqType] {
			t.Errorf("分发表里的 %q 不是 internal/ipc 定义的请求类型", reqType)
		}
	}
	for reqType := range ipcRequestTypesWithoutCoreHandler {
		if !known[reqType] {
			t.Errorf("豁免清单里的 %q 不是 internal/ipc 定义的请求类型", reqType)
		}
	}
}

// TestHandleIPCRequestUnknownType 固定未知请求类型的响应内容。
func TestHandleIPCRequestUnknownType(t *testing.T) {
	app := &CoreApp{}
	resp := app.handleIPCRequest(ipc.Request{Type: ipc.RequestType("NoSuchRequest")})
	if resp.Success {
		t.Fatal("未知请求类型不应返回成功")
	}
	if want := "未知的请求类型: NoSuchRequest"; resp.Error != want {
		t.Fatalf("错误信息 = %q，期望 %q", resp.Error, want)
	}
}
