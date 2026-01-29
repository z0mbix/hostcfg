# Variables

## Defining Variables

Define variables with optional defaults:

```hcl
variable "environment" {
  default     = "production"
  description = "Deployment environment"
}

variable "app_port" {
  default = "8080"
}
```

## Using Variables

Use variables with the `var.` prefix:

```hcl
resource "file" "config" {
  path    = "/etc/myapp/config.conf"
  content = "environment = ${var.environment}\nport = ${var.app_port}\n"
}
```

## Overriding Variables

Override variables from the command line using `-e`:

```bash
hostcfg apply -e environment=staging -e app_port=9090
```

## Variable Precedence

When using roles, variables are resolved with the following precedence (highest to lowest):

1. **CLI variables** (`-e port=6380`) - override everything
2. **Role instantiation variables** (`variables = { ... }`)
3. **Role defaults** (`roles/<name>/defaults/variables.hcl`)
4. **Main config defaults** (`variable "x" { default = ... }`)

## Dependencies

Dependencies are **automatically inferred** from resource references. You can also declare explicit dependencies using `depends_on`:

```hcl
# Automatic dependency - inferred from ${directory.app.path}
resource "directory" "app" {
  path = "/opt/myapp"
}

resource "file" "config" {
  path    = "${directory.app.path}/config.json"  # Auto-depends on directory.app
  content = "{}"
}

# Explicit dependency - needed when no attribute reference exists
resource "package" "nginx" {
  name = "nginx"
}

resource "service" "nginx" {
  name       = "nginx"
  ensure     = "running"
  depends_on = ["package.nginx"]  # Must be explicit
}
```

Dependencies are specified as `["type.name"]` references. The engine performs topological sorting to ensure resources are applied in the correct order and detects circular dependencies.
