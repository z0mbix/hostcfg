# Variables

## Defining Variables

Define variables with optional type constraints and defaults:

```hcl
variable "environment" {
  type        = string
  default     = "production"
  description = "Deployment environment"
}

variable "app_port" {
  type    = number
  default = 8080
}

variable "debug" {
  type    = bool
  default = false
}
```

## Type Constraints

Variables support Terraform-style type constraints. When a type is specified, values are validated at parse time.

### Basic Types

```hcl
variable "name" {
  type    = string
  default = "myapp"
}

variable "port" {
  type    = number
  default = 8080
}

variable "enabled" {
  type    = bool
  default = true
}
```

### Complex Types

```hcl
variable "hosts" {
  type    = list(string)
  default = ["localhost"]
}

variable "ports" {
  type    = set(number)
  default = [80, 443]
}

variable "tags" {
  type = map(string)
  default = {
    env  = "production"
    team = "platform"
  }
}

variable "config" {
  type = object({
    debug     = bool
    log_level = string
    workers   = number
  })
  default = {
    debug     = false
    log_level = "info"
    workers   = 4
  }
}
```

### Nested Types

```hcl
variable "servers" {
  type = list(object({
    host = string
    port = number
  }))
  default = [
    { host = "web1", port = 8080 },
    { host = "web2", port = 8081 }
  ]
}
```

### Type Validation

If a value doesn't match the declared type, hostcfg reports a clear error:

```
Error: Invalid value for variable "port"

The given value is not valid for variable "port": a number is required.

Expected type: number
Given type: string
```

### Untyped Variables

Variables without a `type` attribute accept any value (backward compatible with existing configs):

```hcl
variable "flexible" {
  default = "can be anything"
}
```

## Using Variables

Use variables with the `var.` prefix (see also [System Facts](facts.md) for the `fact.` namespace):

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

### CLI Type Coercion

When a variable has a declared type, CLI string values are automatically coerced:

```bash
# Boolean coercion (accepts: true/false, yes/no, on/off, 1/0)
hostcfg apply -e debug=true
hostcfg apply -e enabled=no

# Number coercion
hostcfg apply -e port=8080
hostcfg apply -e timeout=30.5

# Complex types via JSON
hostcfg apply -e 'hosts=["web1","web2","web3"]'
hostcfg apply -e 'tags={"env":"prod","team":"platform"}'
```

If coercion fails, you'll get a clear error:

```bash
$ hostcfg apply -e port=notanumber
Error: Invalid value for variable "port"
Cannot convert "notanumber" to number: not a valid numeric value
```

## Variable Precedence

When using roles, variables are resolved with the following precedence (highest to lowest):

1. **CLI variables** (`-e port=6380`) - override everything
2. **Role instantiation variables** (`variables = { ... }`)
3. **Role defaults** (`roles/<name>/variables.hcl`)
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
