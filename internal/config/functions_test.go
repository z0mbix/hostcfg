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
