# hostcfg

A Go-based, idempotent configuration management tool using HCL2 syntax.

## Features

- **Idempotent operations** - Resources are only modified when there's actual drift from desired state
- **HCL2 syntax** - Familiar, readable configuration language with variable interpolation
- **Dependency management** - Automatic ordering via `depends_on` with cycle detection
- **Colored diff output** - Clear visualization of planned changes before applying
- **Dry-run support** - Preview changes without modifying the system
- **Cross-platform package management** - Automatic detection of apt, dnf, yum, or pacman

## Installation

```bash
go install github.com/z0mbix/hostcfg/cmd/hostcfg@latest
```

Or build from source:

```bash
git clone https://github.com/z0mbix/hostcfg.git
cd hostcfg
go build -o hostcfg ./cmd/hostcfg
```

## Quick Start

Create a configuration file `hostcfg.hcl`:

```hcl
variable "app_name" {
  default = "myapp"
}

resource "directory" "config" {
  path = "/etc/myapp"
  mode = "0755"
}

resource "file" "config" {
  path    = "/etc/myapp/config.json"
  content = <<-EOF
    {
      "name": "${var.app_name}",
      "version": "1.0.0"
    }
  EOF
  mode = "0644"

  depends_on = ["directory.config"]
}
```

Preview changes:

```bash
hostcfg plan
```

Apply changes:

```bash
hostcfg apply
```

## CLI Commands

### plan

Show what changes would be made without applying them.

```bash
hostcfg plan
hostcfg plan -c /path/to/config.hcl
hostcfg plan -e app_name=customapp
```

### apply

Apply changes to bring the system to the desired state.

```bash
hostcfg apply                    # Interactive confirmation
hostcfg apply --auto-approve     # Skip confirmation
hostcfg apply --dry-run          # Same as plan
```

### validate

Check HCL syntax and validate resource configurations.

```bash
hostcfg validate
hostcfg validate -c /path/to/config.hcl
```

### Global Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--config` | `-c` | Path to config file or directory (default: `hostcfg.hcl` or current directory) |
| `--var` | `-e` | Set a variable (can be used multiple times): `-e key=value` |
| `--no-color` | | Disable colored output |

## Configuration Files

hostcfg supports both single-file and multi-file configurations.

### Single File

```bash
hostcfg plan -c config.hcl
```

### Directory of Files

Load all `*.hcl` files from a directory and merge them:

```bash
hostcfg plan -c /etc/hostcfg/
```

This allows you to organize configuration by concern:

```
/etc/hostcfg/
├── variables.hcl      # Variable definitions
├── packages.hcl       # Package resources
├── services.hcl       # Service resources
├── files.hcl          # File and directory resources
└── cron.hcl           # Cron job resources
```

### Default Behavior

When no `-c` flag is specified, hostcfg looks for configuration in this order:

1. `hostcfg.hcl` in the current directory
2. All `*.hcl` files in the current directory

## Resources

### file

Manages files with content, ownership, and permissions.

```hcl
resource "file" "example" {
  path    = "/etc/myapp/config.conf"
  content = "key = value\n"
  owner   = "root"
  group   = "root"
  mode    = "0644"
  ensure  = "present"  # or "absent"
}
```

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | yes | Absolute path to the file |
| `content` | string | yes* | File content (mutually exclusive with `source`) |
| `source` | string | yes* | Path to source file to copy (mutually exclusive with `content`) |
| `owner` | string | no | File owner username |
| `group` | string | no | File group name |
| `mode` | string | no | File permissions in octal (default: `0644`) |
| `ensure` | string | no | `present` (default) or `absent` |

**Idempotency**: Checks SHA256 hash of content and file stat for ownership/permissions.

### directory

Manages directories with ownership and permissions.

```hcl
resource "directory" "example" {
  path      = "/var/lib/myapp"
  owner     = "appuser"
  group     = "appgroup"
  mode      = "0750"
  recursive = true
  ensure    = "present"  # or "absent"
}
```

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | yes | Absolute path to the directory |
| `owner` | string | no | Directory owner username |
| `group` | string | no | Directory group name |
| `mode` | string | no | Directory permissions in octal (default: `0755`) |
| `recursive` | bool | no | Create parent directories / apply ownership recursively |
| `ensure` | string | no | `present` (default) or `absent` |

**Idempotency**: Checks directory existence and stat for ownership/permissions.

### exec

Executes commands with conditional guards for idempotency.

```hcl
resource "exec" "example" {
  command = "make install"
  dir     = "/opt/myapp"
  user    = "root"
  creates = "/usr/local/bin/myapp"  # Only run if this file doesn't exist
  only_if = "test -f Makefile"      # Only run if this command succeeds
  unless  = "which myapp"           # Don't run if this command succeeds
}
```

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | string | yes | Command to execute (run via `sh -c`) |
| `dir` | string | no | Working directory for the command |
| `user` | string | no | User to run the command as |
| `creates` | string | no | Path to a file; command only runs if file doesn't exist |
| `only_if` | string | no | Guard command; main command only runs if this succeeds |
| `unless` | string | no | Guard command; main command only runs if this fails |

**Idempotency**: Use `creates`, `only_if`, or `unless` to make exec resources idempotent.

### package

Manages system packages with automatic package manager detection.

```hcl
resource "package" "nginx" {
  name    = "nginx"
  version = "1.18.0"  # Optional: specific version
  ensure  = "present" # or "absent"
}
```

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | yes | Package name |
| `version` | string | no | Specific version to install |
| `ensure` | string | no | `present` (default) or `absent` |

**Supported package managers** (auto-detected):
- `apt` (Debian, Ubuntu)
- `dnf` (Fedora, RHEL 8+)
- `yum` (RHEL 7, CentOS)
- `pacman` (Arch Linux)

**Idempotency**: Queries package manager to check if package is installed and at correct version.

### service

Manages systemd services.

```hcl
resource "service" "nginx" {
  name    = "nginx"
  ensure  = "running"  # or "stopped"
  enabled = true       # Start on boot

  depends_on = ["package.nginx"]
}
```

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | yes | Service name (without `.service` suffix) |
| `ensure` | string | no | `running` or `stopped` |
| `enabled` | bool | no | Whether the service starts on boot |

**Idempotency**: Uses `systemctl is-active` and `systemctl is-enabled` to check current state.

### cron

Manages cron jobs.

```hcl
resource "cron" "backup" {
  command  = "/usr/local/bin/backup.sh >> /var/log/backup.log 2>&1"
  schedule = "0 2 * * *"  # Daily at 2am
  user     = "root"
  ensure   = "present"    # or "absent"
}
```

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | string | yes | Command to execute |
| `schedule` | string | yes | Cron schedule expression (5 fields: min hour dom mon dow) |
| `user` | string | no | User whose crontab to modify (default: current user) |
| `ensure` | string | no | `present` (default) or `absent` |

**Idempotency**: Parses user's crontab and looks for managed entries by marker comment.

### hostname

Manages the system hostname.

```hcl
resource "hostname" "main" {
  name = "webserver01.example.com"
}
```

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | yes | Desired hostname |

**Idempotency**: Reads `/etc/hostname` and compares with desired value.

## Variables

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

Use variables with the `var.` prefix:

```hcl
resource "file" "config" {
  path    = "/etc/myapp/config.conf"
  content = "environment = ${var.environment}\nport = ${var.app_port}\n"
}
```

Override variables from the command line:

```bash
hostcfg apply -e environment=staging -e app_port=9090
```

## Built-in Functions

### String Functions

| Function | Description | Example |
|----------|-------------|---------|
| `upper(string)` | Convert to uppercase | `upper("hello")` → `"HELLO"` |
| `lower(string)` | Convert to lowercase | `lower("HELLO")` → `"hello"` |
| `trim(string, chars)` | Trim characters from both ends | `trim("  hello  ", " ")` → `"hello"` |
| `trimspace(string)` | Trim whitespace | `trimspace("  hello  ")` → `"hello"` |
| `replace(string, old, new)` | Replace substrings | `replace("hello", "l", "L")` → `"heLLo"` |
| `substr(string, offset, length)` | Extract substring | `substr("hello", 0, 2)` → `"he"` |
| `join(separator, list)` | Join list elements | `join(",", ["a", "b"])` → `"a,b"` |
| `split(separator, string)` | Split string into list | `split(",", "a,b")` → `["a", "b"]` |
| `format(spec, args...)` | Printf-style formatting | `format("Hello %s", "world")` |

### Collection Functions

| Function | Description |
|----------|-------------|
| `length(collection)` | Return length of list, map, or string |
| `contains(list, value)` | Check if list contains value |
| `concat(list1, list2, ...)` | Concatenate lists |
| `keys(map)` | Return map keys |
| `values(map)` | Return map values |
| `merge(map1, map2, ...)` | Merge maps |
| `sort(list)` | Sort a list of strings |
| `distinct(list)` | Remove duplicates |
| `flatten(list)` | Flatten nested lists |
| `reverse(list)` | Reverse a list |

### Numeric Functions

| Function | Description |
|----------|-------------|
| `abs(number)` | Absolute value |
| `ceil(number)` | Round up |
| `floor(number)` | Round down |
| `max(numbers...)` | Maximum value |
| `min(numbers...)` | Minimum value |

### Filesystem Functions

| Function | Description | Example |
|----------|-------------|---------|
| `file(path)` | Read file contents | `file("/etc/hostname")` |
| `basename(path)` | Get filename from path | `basename("/etc/nginx/nginx.conf")` → `"nginx.conf"` |
| `dirname(path)` | Get directory from path | `dirname("/etc/nginx/nginx.conf")` → `"/etc/nginx"` |

### Other Functions

| Function | Description | Example |
|----------|-------------|---------|
| `env(name)` | Get environment variable | `env("HOME")` |
| `coalesce(values...)` | Return first non-null value | `coalesce(var.custom, "default")` |

## Resource References

Resources can reference attributes of other resources using the syntax `resource_type.resource_name.attribute`. **Dependencies are automatically inferred** from these references - no explicit `depends_on` needed:

```hcl
resource "directory" "app" {
  path = "/opt/myapp"
  mode = "0755"
}

resource "directory" "config" {
  path = "${directory.app.path}/config"  # Automatically depends on directory.app
  mode = "0750"
}

resource "file" "settings" {
  path    = "${directory.config.path}/settings.json"  # Automatically depends on directory.config
  content = "{}"
  mode    = "0644"
}
```

### Available Attributes by Resource Type

| Resource | Available Attributes |
|----------|---------------------|
| `file` | `path`, `content`, `mode`, `owner`, `group` |
| `directory` | `path`, `mode`, `owner`, `group` |
| `exec` | `command`, `creates`, `dir` |
| `hostname` | `name` |
| `cron` | `command`, `schedule`, `user` |
| `package` | `name`, `version` |
| `service` | `name` |

## Dependencies

Dependencies are **automatically inferred** from resource references. However, you can also declare explicit dependencies using `depends_on` when there's no attribute reference:

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
  name    = "nginx"
  ensure  = "running"
  depends_on = ["package.nginx"]  # Must be explicit - no attribute reference
}

resource "service" "app" {
  name   = "myapp"
  ensure = "running"

  depends_on = ["file.config"]
}
```

Dependencies are specified as `["type.name"]` references. The engine performs topological sorting to ensure resources are applied in the correct order and detects circular dependencies.

## Example Output

```
$ hostcfg plan

~ file.app_config
    ~ mode: "0600" => "0644"
    ~ content: (changed)
        {
      -   "debug": true,
      +   "debug": false,
          "version": "1.0.0"
        }

+ file.new_config
    + path = "/etc/myapp/new.conf"
    + owner = "root"
    + mode = "0644"

- file.old_config
    - path = "/etc/myapp/old.conf"

Plan: 1 to add, 1 to change, 1 to destroy.
```

## Examples

See the [examples/](examples/) directory for complete configuration examples:

- `basic.hcl` - Simple file and directory management
- `webserver.hcl` - Configure nginx with virtual host
- `cron.hcl` - Set up backup scripts and cron jobs
- `complete.hcl` - Comprehensive example using all resource types

## License

MIT
