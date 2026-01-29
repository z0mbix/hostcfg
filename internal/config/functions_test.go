package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zclconf/go-cty/cty"
)

func TestStandardFunctions(t *testing.T) {
	funcs := standardFunctions()
	if funcs == nil {
		t.Fatal("standardFunctions returned nil")
	}

	// Check that expected functions exist
	// Note: template() is not in standardFunctions() - it's added dynamically
	// in buildEvalContext() with access to context variables
	expectedFuncs := []string{
		// String functions
		"upper", "lower", "trim", "trimprefix", "trimsuffix", "trimspace",
		"replace", "substr", "join", "split", "format", "formatlist",
		// Collection functions
		"length", "coalesce", "concat", "contains", "distinct", "flatten",
		"keys", "values", "merge", "reverse", "sort",
		// Numeric functions
		"abs", "ceil", "floor", "max", "min",
		// Boolean functions
		"not", "and", "or",
		// Type conversion
		"tostring", "tonumber", "tobool",
		// Custom functions
		"env", "file", "basename", "dirname",
	}

	for _, name := range expectedFuncs {
		if _, ok := funcs[name]; !ok {
			t.Errorf("expected function %q not found", name)
		}
	}
}

func TestEnvFunc(t *testing.T) {
	// Set an environment variable for testing
	os.Setenv("HOSTCFG_TEST_VAR", "test_value")
	defer os.Unsetenv("HOSTCFG_TEST_VAR")

	result, err := envFunc.Call([]cty.Value{cty.StringVal("HOSTCFG_TEST_VAR")})
	if err != nil {
		t.Fatalf("envFunc failed: %v", err)
	}

	if result.AsString() != "test_value" {
		t.Errorf("expected 'test_value', got %q", result.AsString())
	}
}

func TestEnvFunc_NonExistent(t *testing.T) {
	// Ensure variable doesn't exist
	os.Unsetenv("HOSTCFG_NONEXISTENT_VAR")

	result, err := envFunc.Call([]cty.Value{cty.StringVal("HOSTCFG_NONEXISTENT_VAR")})
	if err != nil {
		t.Fatalf("envFunc failed: %v", err)
	}

	// Non-existent env var returns empty string
	if result.AsString() != "" {
		t.Errorf("expected empty string for non-existent env var, got %q", result.AsString())
	}
}

func TestFileFunc(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	content := "test content"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	result, err := fileFunc.Call([]cty.Value{cty.StringVal(filePath)})
	if err != nil {
		t.Fatalf("fileFunc failed: %v", err)
	}

	if result.AsString() != content {
		t.Errorf("expected %q, got %q", content, result.AsString())
	}
}

func TestFileFunc_TrimsNewline(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	if err := os.WriteFile(filePath, []byte("content\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	result, err := fileFunc.Call([]cty.Value{cty.StringVal(filePath)})
	if err != nil {
		t.Fatalf("fileFunc failed: %v", err)
	}

	// Trailing newline should be trimmed
	if result.AsString() != "content" {
		t.Errorf("expected 'content', got %q", result.AsString())
	}
}

func TestFileFunc_NonExistent(t *testing.T) {
	_, err := fileFunc.Call([]cty.Value{cty.StringVal("/nonexistent/file.txt")})
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestBasenameFunc(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/path/to/file.txt", "file.txt"},
		{"/path/to/dir", "dir"},
		{"/path/to/dir/", "dir"},
		{"file.txt", "file.txt"},
		{"/", "/"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result, err := basenameFunc.Call([]cty.Value{cty.StringVal(tt.path)})
			if err != nil {
				t.Fatalf("basenameFunc failed: %v", err)
			}
			if result.AsString() != tt.want {
				t.Errorf("basename(%q) = %q, want %q", tt.path, result.AsString(), tt.want)
			}
		})
	}
}

func TestDirnameFunc(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/path/to/file.txt", "/path/to"},
		{"/path/to/dir", "/path/to"},
		{"/path/to/dir/", "/path/to/dir"},
		{"file.txt", "."},
		{"/", "/"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result, err := dirnameFunc.Call([]cty.Value{cty.StringVal(tt.path)})
			if err != nil {
				t.Fatalf("dirnameFunc failed: %v", err)
			}
			if result.AsString() != tt.want {
				t.Errorf("dirname(%q) = %q, want %q", tt.path, result.AsString(), tt.want)
			}
		})
	}
}

func TestStdlibFunctions(t *testing.T) {
	funcs := standardFunctions()

	t.Run("upper", func(t *testing.T) {
		result, err := funcs["upper"].Call([]cty.Value{cty.StringVal("hello")})
		if err != nil {
			t.Fatalf("upper failed: %v", err)
		}
		if result.AsString() != "HELLO" {
			t.Errorf("expected 'HELLO', got %q", result.AsString())
		}
	})

	t.Run("lower", func(t *testing.T) {
		result, err := funcs["lower"].Call([]cty.Value{cty.StringVal("HELLO")})
		if err != nil {
			t.Fatalf("lower failed: %v", err)
		}
		if result.AsString() != "hello" {
			t.Errorf("expected 'hello', got %q", result.AsString())
		}
	})

	t.Run("trimspace", func(t *testing.T) {
		result, err := funcs["trimspace"].Call([]cty.Value{cty.StringVal("  hello  ")})
		if err != nil {
			t.Fatalf("trimspace failed: %v", err)
		}
		if result.AsString() != "hello" {
			t.Errorf("expected 'hello', got %q", result.AsString())
		}
	})

	t.Run("length", func(t *testing.T) {
		list := cty.ListVal([]cty.Value{cty.StringVal("a"), cty.StringVal("b"), cty.StringVal("c")})
		result, err := funcs["length"].Call([]cty.Value{list})
		if err != nil {
			t.Fatalf("length failed: %v", err)
		}
		if !result.Equals(cty.NumberIntVal(3)).True() {
			t.Errorf("expected 3, got %v", result.AsBigFloat())
		}
	})

	t.Run("join", func(t *testing.T) {
		list := cty.ListVal([]cty.Value{cty.StringVal("a"), cty.StringVal("b"), cty.StringVal("c")})
		result, err := funcs["join"].Call([]cty.Value{cty.StringVal(","), list})
		if err != nil {
			t.Fatalf("join failed: %v", err)
		}
		if result.AsString() != "a,b,c" {
			t.Errorf("expected 'a,b,c', got %q", result.AsString())
		}
	})
}

func TestTemplateFunc(t *testing.T) {
	tmpDir := t.TempDir()
	tmplPath := filepath.Join(tmpDir, "test.tpl")

	// Create a simple template file
	if err := os.WriteFile(tmplPath, []byte("Hello, World!"), 0644); err != nil {
		t.Fatalf("failed to write template file: %v", err)
	}

	// Create template function with empty context (absolute path, no baseDir needed)
	tmplFunc := makeTemplateFunc(map[string]cty.Value{}, "")

	result, err := tmplFunc.Call([]cty.Value{cty.StringVal(tmplPath)})
	if err != nil {
		t.Fatalf("template failed: %v", err)
	}

	if result.AsString() != "Hello, World!" {
		t.Errorf("expected 'Hello, World!', got %q", result.AsString())
	}
}

func TestTemplateFunc_WithVariables(t *testing.T) {
	tmpDir := t.TempDir()
	tmplPath := filepath.Join(tmpDir, "test.tpl")

	// Create a template file that uses variables
	tmplContent := `APP_NAME={{ .var.app_name }}
PORT={{ .var.port }}
CONFIG_DIR={{ .directory.config_dir.path }}`
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0644); err != nil {
		t.Fatalf("failed to write template file: %v", err)
	}

	// Create context variables like the parser would
	ctxVars := map[string]cty.Value{
		"var": cty.ObjectVal(map[string]cty.Value{
			"app_name": cty.StringVal("myapp"),
			"port":     cty.StringVal("8080"),
		}),
		"directory": cty.ObjectVal(map[string]cty.Value{
			"config_dir": cty.ObjectVal(map[string]cty.Value{
				"path": cty.StringVal("/opt/myapp/config"),
			}),
		}),
	}

	tmplFunc := makeTemplateFunc(ctxVars, "")

	result, err := tmplFunc.Call([]cty.Value{cty.StringVal(tmplPath)})
	if err != nil {
		t.Fatalf("template failed: %v", err)
	}

	expected := `APP_NAME=myapp
PORT=8080
CONFIG_DIR=/opt/myapp/config`
	if result.AsString() != expected {
		t.Errorf("expected:\n%s\n\ngot:\n%s", expected, result.AsString())
	}
}

func TestTemplateFunc_MissingFile(t *testing.T) {
	tmplFunc := makeTemplateFunc(map[string]cty.Value{}, "")

	_, err := tmplFunc.Call([]cty.Value{cty.StringVal("/nonexistent/template.tpl")})
	if err == nil {
		t.Error("expected error for non-existent template file")
	}
}

func TestTemplateFunc_InvalidSyntax(t *testing.T) {
	tmpDir := t.TempDir()
	tmplPath := filepath.Join(tmpDir, "invalid.tpl")

	// Create a template with invalid syntax
	if err := os.WriteFile(tmplPath, []byte("{{ .invalid syntax }}"), 0644); err != nil {
		t.Fatalf("failed to write template file: %v", err)
	}

	tmplFunc := makeTemplateFunc(map[string]cty.Value{}, "")

	_, err := tmplFunc.Call([]cty.Value{cty.StringVal(tmplPath)})
	if err == nil {
		t.Error("expected error for invalid template syntax")
	}
}

func TestTemplateFunc_RelativePath(t *testing.T) {
	tmpDir := t.TempDir()
	tmplPath := filepath.Join(tmpDir, "test.tpl")

	// Create a simple template file
	if err := os.WriteFile(tmplPath, []byte("Hello from relative path!"), 0644); err != nil {
		t.Fatalf("failed to write template file: %v", err)
	}

	// Create template function with baseDir set
	tmplFunc := makeTemplateFunc(map[string]cty.Value{}, tmpDir)

	// Use relative path (just the filename)
	result, err := tmplFunc.Call([]cty.Value{cty.StringVal("test.tpl")})
	if err != nil {
		t.Fatalf("template failed: %v", err)
	}

	if result.AsString() != "Hello from relative path!" {
		t.Errorf("expected 'Hello from relative path!', got %q", result.AsString())
	}
}

func TestTemplateFunc_RelativePathWithDotSlash(t *testing.T) {
	tmpDir := t.TempDir()
	tmplPath := filepath.Join(tmpDir, "test.tpl")

	// Create a simple template file
	if err := os.WriteFile(tmplPath, []byte("Hello with ./!"), 0644); err != nil {
		t.Fatalf("failed to write template file: %v", err)
	}

	// Create template function with baseDir set
	tmplFunc := makeTemplateFunc(map[string]cty.Value{}, tmpDir)

	// Use relative path with ./
	result, err := tmplFunc.Call([]cty.Value{cty.StringVal("./test.tpl")})
	if err != nil {
		t.Fatalf("template failed: %v", err)
	}

	if result.AsString() != "Hello with ./!" {
		t.Errorf("expected 'Hello with ./!', got %q", result.AsString())
	}
}

func TestCtyToGoMap(t *testing.T) {
	// Test with object value
	objVal := cty.ObjectVal(map[string]cty.Value{
		"string": cty.StringVal("hello"),
		"number": cty.NumberIntVal(42),
		"bool":   cty.BoolVal(true),
	})

	result := ctyToGoMap(objVal)

	if result["string"] != "hello" {
		t.Errorf("expected string 'hello', got %v", result["string"])
	}
	if result["number"] != float64(42) {
		t.Errorf("expected number 42, got %v", result["number"])
	}
	if result["bool"] != true {
		t.Errorf("expected bool true, got %v", result["bool"])
	}
}

func TestCtyToGoMap_Null(t *testing.T) {
	result := ctyToGoMap(cty.NullVal(cty.String))
	if len(result) != 0 {
		t.Errorf("expected empty map for null value, got %v", result)
	}
}

func TestCtyToInterface_List(t *testing.T) {
	listVal := cty.ListVal([]cty.Value{
		cty.StringVal("a"),
		cty.StringVal("b"),
		cty.StringVal("c"),
	})

	result := ctyToInterface(listVal)
	list, ok := result.([]interface{})
	if !ok {
		t.Fatalf("expected []interface{}, got %T", result)
	}

	if len(list) != 3 {
		t.Errorf("expected 3 elements, got %d", len(list))
	}
	if list[0] != "a" || list[1] != "b" || list[2] != "c" {
		t.Errorf("unexpected list contents: %v", list)
	}
}
