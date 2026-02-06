# hostcfg

A simple, idempotent configuration management tool using HCL syntax.

## Features

- **Idempotent** - Resources only change when there's drift from desired state
- **HCL syntax** - Familiar configuration language with variable interpolation
- **Typed variables** - Terraform-style type constraints with automatic CLI coercion
- **Dependency management** - Automatic ordering with cycle detection
- **Roles** - Reusable configuration modules with variables and templates
- **System facts** - Ansible-style facts for OS, architecture, and user info
- **Diff output** - Clear visualization of planned changes
- **Cross-platform** - Supports Linux, macOS, and BSD systems

## Supported Platforms

| Platform | Package Manager | Service Manager |
|----------|----------------|-----------------|
| macOS | Homebrew | launchctl |
| Debian/Ubuntu | apt | systemd |
| Fedora/RHEL 8+ | dnf | systemd |
| RHEL 7/CentOS | yum | systemd |
| Arch Linux | pacman | systemd |
| FreeBSD | pkg | rc.d |
| OpenBSD | pkg_add | rcctl |
| NetBSD | pkgin, pkg_add | rc.d |
| SmartOS | pkgin | SMF |
| OmniOS | pkg | SMF |

## Installation

Download the latest binary for your platform from the [releases page](https://github.com/z0mbix/hostcfg/releases).

Or build from source:

```bash
git clone https://github.com/z0mbix/hostcfg.git
cd hostcfg
go build -o hostcfg ./cmd/hostcfg
```

## Quick Start

Create `hostcfg.hcl`:

```hcl
variable "app_name" {
  type    = string
  default = "myapp"
}

variable "debug" {
  type    = bool
  default = false
}

# Create app config directory in user's home (using a system fact)
resource "directory" "config" {
  description = "Create app config directory"
  path        = "${fact.user.home}/.config/${var.app_name}"
  mode        = "0755"
}

# Create multiple config files with for_each
resource "file" "configs" {
  description = "Create config files for ${each.key}"
  for_each = toset(["app", "db", "cache"])
  path     = "${directory.config.path}/${each.key}.conf"
  content  = "# ${each.value} configuration\ndebug = ${var.debug}\n"
}

# Only create this file on Linux (using a when condition)
resource "file" "platform_info" {
  description = "Create platform info file for ${fact.os.name}"
  path    = "${directory.config.path}/platform.txt"
  content = "Running on ${fact.os.distribution} (${fact.arch})\n"

  when = [
    fact.os.name == "darwin" || fact.os.name == "linux",
    var.debug
  ]
}
```

Preview and apply:

```bash
hostcfg plan
hostcfg apply

# Override typed variables from CLI (automatic type coercion)
hostcfg apply -e debug=true -e app_name=otherapp
```

Get system facts:

```bash
hostcfg facts
```

## Documentation

- [CLI Reference](docs/cli.md) - Commands and flags
- [Resources](docs/resources.md) - All resource types and attributes
- [Variables](docs/variables.md) - Variable definitions and dependencies
- [Functions](docs/functions.md) - Built-in functions
- [System Facts](docs/facts.md) - OS, architecture, and user information
- [Roles](docs/roles.md) - Reusable configuration modules
- [Playbooks](docs/playbooks.md) - Multi-role configurations

## Resources

| Resource | Description |
|----------|-------------|
| `file` | Manage files with content and permissions |
| `directory` | Manage directories |
| `link` | Manage symbolic links |
| `download` | Download files from URLs with checksum verification |
| `stat` | Gather file/directory information (read-only) |
| `package` | Install/remove system packages |
| `service` | Manage system services |
| `user` | Manage system users |
| `group` | Manage system groups |
| `cron` | Manage cron jobs |
| `exec` | Execute commands with guards |
| `hostname` | Set system hostname |

## Examples

See the [examples/](examples/) directory for complete configurations.

## Development

Install dependencies (requires [mise](https://mise.jdx.dev/)):

```bash
mise install
```

Install the pre-commit hook to run the same checks as CI before each commit:

```bash
cp scripts/pre-commit .git/hooks/pre-commit
```

The hook runs `go mod verify`, `go build`, `golangci-lint`, and `go test -race`.

## License

MIT
