# hostcfg

A simple, idempotent configuration management tool using HCL syntax.

## Features

- **Idempotent** - Resources only change when there's drift from desired state
- **HCL syntax** - Familiar configuration language with variable interpolation
- **Dependency management** - Automatic ordering with cycle detection
- **Roles** - Reusable configuration modules with variables and templates
- **Diff output** - Clear visualization of planned changes
- **Cross-platform** - Supports Linux and BSD systems

## Supported Platforms

| Platform | Package Manager | Service Manager |
|----------|----------------|-----------------|
| Debian/Ubuntu | apt | systemd |
| Fedora/RHEL 8+ | dnf | systemd |
| RHEL 7/CentOS | yum | systemd |
| Arch Linux | pacman | systemd |
| FreeBSD | pkg | rc.d |
| OpenBSD | pkg_add | rcctl |
| NetBSD | pkgin, pkg_add | rc.d |

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

Create `hostcfg.hcl`:

```hcl
resource "directory" "config" {
  path = "/etc/myapp"
  mode = "0755"
}

resource "file" "config" {
  path    = "${directory.config.path}/config.json"
  content = <<-EOF
    {"name": "myapp", "version": "1.0.0"}
  EOF
}
```

Preview and apply:

```bash
hostcfg plan
hostcfg apply
```

## Documentation

- [CLI Reference](docs/cli.md) - Commands and flags
- [Resources](docs/resources.md) - All resource types and attributes
- [Variables](docs/variables.md) - Variable definitions and dependencies
- [Functions](docs/functions.md) - Built-in functions
- [Roles](docs/roles.md) - Reusable configuration modules
- [Playbooks](docs/playbooks.md) - Multi-role configurations

## Resources

| Resource | Description |
|----------|-------------|
| `file` | Manage files with content and permissions |
| `directory` | Manage directories |
| `link` | Manage symbolic links |
| `package` | Install/remove system packages |
| `service` | Manage system services |
| `user` | Manage system users |
| `group` | Manage system groups |
| `cron` | Manage cron jobs |
| `exec` | Execute commands with guards |
| `hostname` | Set system hostname |

## Examples

See the [examples/](examples/) directory for complete configurations.

## License

MIT
