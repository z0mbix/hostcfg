package role

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/z0mbix/hostcfg/internal/config"
	"github.com/zclconf/go-cty/cty"
)

func TestNewLoader(t *testing.T) {
	parser := config.NewParser()
	loader := NewLoader(parser, "/base", nil)

	if loader == nil {
		t.Fatal("NewLoader returned nil")
	}
	if loader.mainParser != parser {
		t.Error("mainParser not set correctly")
	}
	if loader.mainBaseDir != "/base" {
		t.Errorf("mainBaseDir = %q, want /base", loader.mainBaseDir)
	}
	if loader.cliVars == nil {
		t.Error("cliVars should be initialized")
	}
}

func TestLoader_LoadRole_BasicStructure(t *testing.T) {
	tmpDir := t.TempDir()

	// Create role directory structure
	roleDir := filepath.Join(tmpDir, "roles", "redis")
	if err := os.MkdirAll(roleDir, 0755); err != nil {
		t.Fatalf("failed to create role dir: %v", err)
	}

	// Create resources.hcl
	resourcesHCL := `
resource "file" "config" {
  path    = "/etc/redis/redis.conf"
  content = "port 6379"
}
`
	if err := os.WriteFile(filepath.Join(roleDir, "resources.hcl"), []byte(resourcesHCL), 0644); err != nil {
		t.Fatalf("failed to write resources.hcl: %v", err)
	}

	// Create main config
	mainHCL := `
role "redis" {
  source = "./roles/redis"
}
`
	mainPath := filepath.Join(tmpDir, "main.hcl")
	if err := os.WriteFile(mainPath, []byte(mainHCL), 0644); err != nil {
		t.Fatalf("failed to write main.hcl: %v", err)
	}

	parser := config.NewParser()
	cfg, diags := parser.ParseFile(mainPath)
	if diags.HasErrors() {
		t.Fatalf("failed to parse main config: %s", diags.Error())
	}

	loader := NewLoader(parser, tmpDir, nil)
	role, err := loader.LoadRole(cfg.Roles[0])
	if err != nil {
		t.Fatalf("LoadRole failed: %v", err)
	}

	if role.Name != "redis" {
		t.Errorf("role.Name = %q, want redis", role.Name)
	}
	if len(role.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(role.Resources))
	}
	if role.Resources[0].Name != "redis_config" {
		t.Errorf("resource name = %q, want redis_config", role.Resources[0].Name)
	}
}

func TestLoader_LoadRole_DefaultVariables(t *testing.T) {
	tmpDir := t.TempDir()

	// Create role directory structure
	roleDir := filepath.Join(tmpDir, "roles", "redis")
	if err := os.MkdirAll(roleDir, 0755); err != nil {
		t.Fatalf("failed to create role dir: %v", err)
	}

	// Create variables.hcl in role root
	defaultsHCL := `
variable "port" {
  default = 6379
}

variable "maxmemory" {
  default = "256mb"
}
`
	if err := os.WriteFile(filepath.Join(roleDir, "variables.hcl"), []byte(defaultsHCL), 0644); err != nil {
		t.Fatalf("failed to write variables.hcl: %v", err)
	}

	// Create empty resources.hcl
	if err := os.WriteFile(filepath.Join(roleDir, "resources.hcl"), []byte(""), 0644); err != nil {
		t.Fatalf("failed to write resources.hcl: %v", err)
	}

	// Create main config
	mainHCL := `
role "redis" {
  source = "./roles/redis"
}
`
	mainPath := filepath.Join(tmpDir, "main.hcl")
	if err := os.WriteFile(mainPath, []byte(mainHCL), 0644); err != nil {
		t.Fatalf("failed to write main.hcl: %v", err)
	}

	parser := config.NewParser()
	cfg, diags := parser.ParseFile(mainPath)
	if diags.HasErrors() {
		t.Fatalf("failed to parse main config: %s", diags.Error())
	}

	loader := NewLoader(parser, tmpDir, nil)
	role, err := loader.LoadRole(cfg.Roles[0])
	if err != nil {
		t.Fatalf("LoadRole failed: %v", err)
	}

	if !role.Defaults["port"].Equals(cty.NumberIntVal(6379)).True() {
		t.Errorf("default port = %v, want 6379", role.Defaults["port"])
	}
	if role.Defaults["maxmemory"].AsString() != "256mb" {
		t.Errorf("default maxmemory = %v, want 256mb", role.Defaults["maxmemory"])
	}
}

func TestLoader_LoadRole_VariablePrecedence(t *testing.T) {
	tmpDir := t.TempDir()

	// Create role directory structure
	roleDir := filepath.Join(tmpDir, "roles", "redis")
	if err := os.MkdirAll(roleDir, 0755); err != nil {
		t.Fatalf("failed to create role dir: %v", err)
	}

	// Create variables.hcl with port=6379
	defaultsHCL := `
variable "port" {
  default = 6379
}
`
	if err := os.WriteFile(filepath.Join(roleDir, "variables.hcl"), []byte(defaultsHCL), 0644); err != nil {
		t.Fatalf("failed to write variables.hcl: %v", err)
	}

	// Create resources.hcl that uses the port variable
	resourcesHCL := `
resource "file" "config" {
  path    = "/etc/redis/redis.conf"
  content = "port ${var.port}"
}
`
	if err := os.WriteFile(filepath.Join(roleDir, "resources.hcl"), []byte(resourcesHCL), 0644); err != nil {
		t.Fatalf("failed to write resources.hcl: %v", err)
	}

	// Create main config with port=6380 override
	mainHCL := `
role "redis" {
  source = "./roles/redis"

  variables = {
    port = 6380
  }
}
`
	mainPath := filepath.Join(tmpDir, "main.hcl")
	if err := os.WriteFile(mainPath, []byte(mainHCL), 0644); err != nil {
		t.Fatalf("failed to write main.hcl: %v", err)
	}

	parser := config.NewParser()
	cfg, diags := parser.ParseFile(mainPath)
	if diags.HasErrors() {
		t.Fatalf("failed to parse main config: %s", diags.Error())
	}

	loader := NewLoader(parser, tmpDir, nil)
	role, err := loader.LoadRole(cfg.Roles[0])
	if err != nil {
		t.Fatalf("LoadRole failed: %v", err)
	}

	// Instantiation should override defaults
	if !role.Variables["port"].Equals(cty.NumberIntVal(6380)).True() {
		t.Errorf("instantiation port = %v, want 6380", role.Variables["port"])
	}
}

func TestLoader_LoadRole_ResourcePrefixing(t *testing.T) {
	tmpDir := t.TempDir()

	// Create role directory structure
	roleDir := filepath.Join(tmpDir, "roles", "redis")
	if err := os.MkdirAll(roleDir, 0755); err != nil {
		t.Fatalf("failed to create role dir: %v", err)
	}

	// Create resources with multiple resources
	resourcesHCL := `
resource "package" "redis" {
  name = "redis-server"
}

resource "file" "config" {
  path    = "/etc/redis/redis.conf"
  content = "port 6379"
}

resource "service" "redis" {
  name = "redis-server"
}
`
	if err := os.WriteFile(filepath.Join(roleDir, "resources.hcl"), []byte(resourcesHCL), 0644); err != nil {
		t.Fatalf("failed to write resources.hcl: %v", err)
	}

	// Create main config
	mainHCL := `
role "redis" {
  source = "./roles/redis"
}
`
	mainPath := filepath.Join(tmpDir, "main.hcl")
	if err := os.WriteFile(mainPath, []byte(mainHCL), 0644); err != nil {
		t.Fatalf("failed to write main.hcl: %v", err)
	}

	parser := config.NewParser()
	cfg, diags := parser.ParseFile(mainPath)
	if diags.HasErrors() {
		t.Fatalf("failed to parse main config: %s", diags.Error())
	}

	loader := NewLoader(parser, tmpDir, nil)
	role, err := loader.LoadRole(cfg.Roles[0])
	if err != nil {
		t.Fatalf("LoadRole failed: %v", err)
	}

	if len(role.Resources) != 3 {
		t.Fatalf("expected 3 resources, got %d", len(role.Resources))
	}

	// Verify all resources are prefixed
	expectedNames := map[string]bool{
		"redis_redis":  true, // package.redis -> package.redis_redis
		"redis_config": true, // file.config -> file.redis_config
	}
	for _, res := range role.Resources {
		if res.Type == "package" && res.Name != "redis_redis" {
			t.Errorf("package name = %q, want redis_redis", res.Name)
		}
		if res.Type == "file" && res.Name != "redis_config" {
			t.Errorf("file name = %q, want redis_config", res.Name)
		}
		if res.Type == "service" && res.Name != "redis_redis" {
			t.Errorf("service name = %q, want redis_redis", res.Name)
		}
	}
	_ = expectedNames // silence unused warning
}

func TestLoader_LoadRole_DependencyTransformation(t *testing.T) {
	tmpDir := t.TempDir()

	// Create role directory structure
	roleDir := filepath.Join(tmpDir, "roles", "redis")
	if err := os.MkdirAll(roleDir, 0755); err != nil {
		t.Fatalf("failed to create role dir: %v", err)
	}

	// Create resources with internal dependencies
	resourcesHCL := `
resource "package" "redis" {
  name = "redis-server"
}

resource "file" "config" {
  path       = "/etc/redis/redis.conf"
  content    = "port 6379"
  depends_on = ["package.redis"]
}

resource "service" "redis" {
  name       = "redis-server"
  depends_on = ["file.config"]
}
`
	if err := os.WriteFile(filepath.Join(roleDir, "resources.hcl"), []byte(resourcesHCL), 0644); err != nil {
		t.Fatalf("failed to write resources.hcl: %v", err)
	}

	// Create main config
	mainHCL := `
role "redis" {
  source = "./roles/redis"
}
`
	mainPath := filepath.Join(tmpDir, "main.hcl")
	if err := os.WriteFile(mainPath, []byte(mainHCL), 0644); err != nil {
		t.Fatalf("failed to write main.hcl: %v", err)
	}

	parser := config.NewParser()
	cfg, diags := parser.ParseFile(mainPath)
	if diags.HasErrors() {
		t.Fatalf("failed to parse main config: %s", diags.Error())
	}

	loader := NewLoader(parser, tmpDir, nil)
	role, err := loader.LoadRole(cfg.Roles[0])
	if err != nil {
		t.Fatalf("LoadRole failed: %v", err)
	}

	// Find the file resource and check its transformed dependency
	for _, res := range role.Resources {
		if res.Type == "file" {
			if len(res.DependsOn) != 1 || res.DependsOn[0] != "package.redis_redis" {
				t.Errorf("file depends_on = %v, want [package.redis_redis]", res.DependsOn)
			}
		}
		if res.Type == "service" {
			if len(res.DependsOn) != 1 || res.DependsOn[0] != "file.redis_config" {
				t.Errorf("service depends_on = %v, want [file.redis_config]", res.DependsOn)
			}
		}
	}
}

func TestLoader_LoadRole_MissingSource(t *testing.T) {
	tmpDir := t.TempDir()

	// Create main config with non-existent role source
	mainHCL := `
role "missing" {
  source = "./roles/nonexistent"
}
`
	mainPath := filepath.Join(tmpDir, "main.hcl")
	if err := os.WriteFile(mainPath, []byte(mainHCL), 0644); err != nil {
		t.Fatalf("failed to write main.hcl: %v", err)
	}

	parser := config.NewParser()
	cfg, diags := parser.ParseFile(mainPath)
	if diags.HasErrors() {
		t.Fatalf("failed to parse main config: %s", diags.Error())
	}

	loader := NewLoader(parser, tmpDir, nil)
	_, err := loader.LoadRole(cfg.Roles[0])
	if err == nil {
		t.Error("expected error for missing source, got nil")
	}
}

func TestLoader_LoadRole_InvalidHCL(t *testing.T) {
	tmpDir := t.TempDir()

	// Create role directory structure
	roleDir := filepath.Join(tmpDir, "roles", "broken")
	if err := os.MkdirAll(roleDir, 0755); err != nil {
		t.Fatalf("failed to create role dir: %v", err)
	}

	// Create invalid HCL
	invalidHCL := `this is not valid { HCL`
	if err := os.WriteFile(filepath.Join(roleDir, "resources.hcl"), []byte(invalidHCL), 0644); err != nil {
		t.Fatalf("failed to write resources.hcl: %v", err)
	}

	// Create main config
	mainHCL := `
role "broken" {
  source = "./roles/broken"
}
`
	mainPath := filepath.Join(tmpDir, "main.hcl")
	if err := os.WriteFile(mainPath, []byte(mainHCL), 0644); err != nil {
		t.Fatalf("failed to write main.hcl: %v", err)
	}

	parser := config.NewParser()
	cfg, diags := parser.ParseFile(mainPath)
	if diags.HasErrors() {
		t.Fatalf("failed to parse main config: %s", diags.Error())
	}

	loader := NewLoader(parser, tmpDir, nil)
	_, err := loader.LoadRole(cfg.Roles[0])
	if err == nil {
		t.Error("expected error for invalid HCL, got nil")
	}
}

func TestLoader_LoadRole_NoDefaultsFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create role directory structure without defaults
	roleDir := filepath.Join(tmpDir, "roles", "simple")
	if err := os.MkdirAll(roleDir, 0755); err != nil {
		t.Fatalf("failed to create role dir: %v", err)
	}

	// Create simple resources.hcl
	resourcesHCL := `
resource "file" "test" {
  path    = "/tmp/test"
  content = "test"
}
`
	if err := os.WriteFile(filepath.Join(roleDir, "resources.hcl"), []byte(resourcesHCL), 0644); err != nil {
		t.Fatalf("failed to write resources.hcl: %v", err)
	}

	// Create main config
	mainHCL := `
role "simple" {
  source = "./roles/simple"
}
`
	mainPath := filepath.Join(tmpDir, "main.hcl")
	if err := os.WriteFile(mainPath, []byte(mainHCL), 0644); err != nil {
		t.Fatalf("failed to write main.hcl: %v", err)
	}

	parser := config.NewParser()
	cfg, diags := parser.ParseFile(mainPath)
	if diags.HasErrors() {
		t.Fatalf("failed to parse main config: %s", diags.Error())
	}

	loader := NewLoader(parser, tmpDir, nil)
	role, err := loader.LoadRole(cfg.Roles[0])
	if err != nil {
		t.Fatalf("LoadRole failed: %v", err)
	}

	// Should succeed with empty defaults
	if len(role.Defaults) != 0 {
		t.Errorf("expected 0 defaults, got %d", len(role.Defaults))
	}
}

func TestLoader_MultipleRoles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create redis role
	redisDir := filepath.Join(tmpDir, "roles", "redis")
	if err := os.MkdirAll(redisDir, 0755); err != nil {
		t.Fatalf("failed to create redis role dir: %v", err)
	}
	redisHCL := `
resource "service" "redis" {
  name = "redis-server"
}
`
	if err := os.WriteFile(filepath.Join(redisDir, "resources.hcl"), []byte(redisHCL), 0644); err != nil {
		t.Fatalf("failed to write redis resources.hcl: %v", err)
	}

	// Create webapp role
	webappDir := filepath.Join(tmpDir, "roles", "webapp")
	if err := os.MkdirAll(webappDir, 0755); err != nil {
		t.Fatalf("failed to create webapp role dir: %v", err)
	}
	webappHCL := `
resource "file" "config" {
  path    = "/etc/webapp/config"
  content = "webapp config"
}
`
	if err := os.WriteFile(filepath.Join(webappDir, "resources.hcl"), []byte(webappHCL), 0644); err != nil {
		t.Fatalf("failed to write webapp resources.hcl: %v", err)
	}

	// Create main config with both roles
	mainHCL := `
role "redis" {
  source = "./roles/redis"
}

role "webapp" {
  source = "./roles/webapp"
}
`
	mainPath := filepath.Join(tmpDir, "main.hcl")
	if err := os.WriteFile(mainPath, []byte(mainHCL), 0644); err != nil {
		t.Fatalf("failed to write main.hcl: %v", err)
	}

	parser := config.NewParser()
	cfg, diags := parser.ParseFile(mainPath)
	if diags.HasErrors() {
		t.Fatalf("failed to parse main config: %s", diags.Error())
	}

	if len(cfg.Roles) != 2 {
		t.Fatalf("expected 2 roles, got %d", len(cfg.Roles))
	}

	loader := NewLoader(parser, tmpDir, nil)

	// Load both roles
	redis, err := loader.LoadRole(cfg.Roles[0])
	if err != nil {
		t.Fatalf("failed to load redis role: %v", err)
	}
	webapp, err := loader.LoadRole(cfg.Roles[1])
	if err != nil {
		t.Fatalf("failed to load webapp role: %v", err)
	}

	if redis.Name != "redis" {
		t.Errorf("redis name = %q", redis.Name)
	}
	if webapp.Name != "webapp" {
		t.Errorf("webapp name = %q", webapp.Name)
	}
	if len(redis.Resources) != 1 {
		t.Errorf("redis resources = %d", len(redis.Resources))
	}
	if len(webapp.Resources) != 1 {
		t.Errorf("webapp resources = %d", len(webapp.Resources))
	}
}

func TestLoader_RoleDependencies(t *testing.T) {
	tmpDir := t.TempDir()

	// Create redis role
	redisDir := filepath.Join(tmpDir, "roles", "redis")
	if err := os.MkdirAll(redisDir, 0755); err != nil {
		t.Fatalf("failed to create redis role dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(redisDir, "resources.hcl"), []byte(`
resource "service" "redis" {
  name = "redis-server"
}
`), 0644); err != nil {
		t.Fatalf("failed to write resources.hcl: %v", err)
	}

	// Create webapp role that depends on redis role
	webappDir := filepath.Join(tmpDir, "roles", "webapp")
	if err := os.MkdirAll(webappDir, 0755); err != nil {
		t.Fatalf("failed to create webapp role dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(webappDir, "resources.hcl"), []byte(`
resource "file" "config" {
  path    = "/etc/webapp/config"
  content = "webapp config"
}
`), 0644); err != nil {
		t.Fatalf("failed to write resources.hcl: %v", err)
	}

	// Create main config with role dependency
	mainHCL := `
role "redis" {
  source = "./roles/redis"
}

role "webapp" {
  source     = "./roles/webapp"
  depends_on = ["role.redis"]
}
`
	mainPath := filepath.Join(tmpDir, "main.hcl")
	if err := os.WriteFile(mainPath, []byte(mainHCL), 0644); err != nil {
		t.Fatalf("failed to write main.hcl: %v", err)
	}

	parser := config.NewParser()
	cfg, diags := parser.ParseFile(mainPath)
	if diags.HasErrors() {
		t.Fatalf("failed to parse main config: %s", diags.Error())
	}

	loader := NewLoader(parser, tmpDir, nil)
	webapp, err := loader.LoadRole(cfg.Roles[1])
	if err != nil {
		t.Fatalf("failed to load webapp role: %v", err)
	}

	if len(webapp.DependsOn) != 1 || webapp.DependsOn[0] != "role.redis" {
		t.Errorf("webapp depends_on = %v, want [role.redis]", webapp.DependsOn)
	}
}

func TestLoader_transformDependencies(t *testing.T) {
	loader := &Loader{}

	tests := []struct {
		name     string
		deps     []string
		roleName string
		want     []string
	}{
		{
			name:     "simple resource deps",
			deps:     []string{"package.redis", "file.config"},
			roleName: "myapp",
			want:     []string{"package.myapp_redis", "file.myapp_config"},
		},
		{
			name:     "role deps preserved",
			deps:     []string{"role.redis"},
			roleName: "webapp",
			want:     []string{"role.redis"},
		},
		{
			name:     "mixed deps",
			deps:     []string{"package.redis", "role.other"},
			roleName: "myapp",
			want:     []string{"package.myapp_redis", "role.other"},
		},
		{
			name:     "empty deps",
			deps:     []string{},
			roleName: "myapp",
			want:     []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := loader.transformDependencies(tt.deps, tt.roleName)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("got[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestLoader_SourceNotDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file instead of directory
	filePath := filepath.Join(tmpDir, "notadir")
	if err := os.WriteFile(filePath, []byte("I'm a file"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Create main config pointing to the file
	mainHCL := `
role "broken" {
  source = "./notadir"
}
`
	mainPath := filepath.Join(tmpDir, "main.hcl")
	if err := os.WriteFile(mainPath, []byte(mainHCL), 0644); err != nil {
		t.Fatalf("failed to write main.hcl: %v", err)
	}

	parser := config.NewParser()
	cfg, diags := parser.ParseFile(mainPath)
	if diags.HasErrors() {
		t.Fatalf("failed to parse main config: %s", diags.Error())
	}

	loader := NewLoader(parser, tmpDir, nil)
	_, err := loader.LoadRole(cfg.Roles[0])
	if err == nil {
		t.Error("expected error for file as role source, got nil")
	}
}
