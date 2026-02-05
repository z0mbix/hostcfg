# System Facts

System facts provide Ansible-style information about the host system. Facts are gathered once at startup and made available in HCL expressions via the `fact` namespace.

## Available Facts

| Fact Path | Description | Example Value |
|-----------|-------------|---------------|
| `fact.os.name` | OS name | `linux`, `darwin` |
| `fact.os.family` | OS family | `debian`, `redhat`, `arch`, `darwin` |
| `fact.os.distribution` | Distribution name | `Ubuntu`, `Fedora`, `Arch Linux` |
| `fact.os.distribution_version` | Distribution version | `22.04`, `39` |
| `fact.arch` | CPU architecture | `amd64`, `arm64` |
| `fact.hostname` | System hostname | `myserver` |
| `fact.fqdn` | Fully qualified domain name | `myserver.example.com` |
| `fact.machine_id` | Machine ID from `/etc/machine-id` (Linux only) | `abc123def456...` |
| `fact.cpu.physical` | Number of physical CPU cores | `4` |
| `fact.cpu.cores` | Total CPU cores (logical/hyperthreaded) | `8` |
| `fact.user.name` | Current username | `admin` |
| `fact.user.home` | Home directory | `/home/admin` |
| `fact.user.uid` | User ID | `1000` |
| `fact.user.gid` | Group ID | `1000` |
| `fact.env.<VAR>` | Environment variable value | `fact.env.HOME` → `/home/admin` |

## Machine ID

The `fact.machine_id` fact provides the system's unique machine identifier, read from `/etc/machine-id` on Linux systems. This is useful for uniquely identifying a machine in configuration or generating machine-specific settings.

```hcl
resource "file" "machine_info" {
  path    = "/etc/myapp/machine.conf"
  content = <<-EOF
    machine_id = ${fact.machine_id}
  EOF
}
```

On non-Linux systems, this value will be an empty string.

## CPU Information

CPU facts provide information about the system's processor configuration:

- `fact.cpu.physical` - Number of physical CPU cores
- `fact.cpu.cores` - Total logical CPU cores (includes hyperthreading)

```hcl
# Configure worker threads based on CPU cores
resource "file" "app_config" {
  path    = "/etc/myapp/workers.conf"
  content = <<-EOF
    # Use half the logical cores for workers
    workers = ${fact.cpu.cores / 2}
  EOF
}
```

## Environment Variables

All environment variables are available via `fact.env.<VARIABLE_NAME>`.

To view facts without environment variables (which can be noisy), use `--no-env`:

```bash
hostcfg facts --no-env
```

```hcl
# Conditional execution based on environment
resource "file" "dev_config" {
  path    = "/etc/myapp/config.conf"
  content = "mode = development"
  when    = fact.env.APP_ENV == "development"
}

# Use environment variables in paths
resource "directory" "workspace" {
  path = "${fact.env.HOME}/workspace"
  mode = "0755"
}
```

## OS Family Detection

The `fact.os.family` value is derived from `/etc/os-release` on Linux systems:

| Family | Distributions |
|--------|---------------|
| `debian` | Debian, Ubuntu, Linux Mint, Pop!_OS, Elementary, Kali, Raspbian |
| `redhat` | Fedora, RHEL, CentOS, Rocky Linux, AlmaLinux, Oracle Linux, Amazon Linux |
| `arch` | Arch Linux, Manjaro, EndeavourOS, Garuda |
| `suse` | openSUSE, SLES |
| `alpine` | Alpine Linux |
| `darwin` | macOS |

## Using Facts in HCL Expressions

Facts can be used anywhere HCL expressions are allowed:

```hcl
# Reference user's home directory
resource "directory" "app_config" {
  path = "${fact.user.home}/.config/myapp"
  mode = "0755"
}

# Conditional package names based on OS family
resource "package" "editor" {
  name = fact.os.family == "debian" ? "vim" : "vim-enhanced"
  ensure = "present"
}

# Use hostname in file content
resource "file" "config" {
  path    = "/etc/myapp/config.conf"
  content = "hostname = ${fact.hostname}\n"
}
```

### Nested Ternary for Multiple OS Families

```hcl
resource "package" "build_tools" {
  name = fact.os.family == "debian" ? "build-essential" : (
    fact.os.family == "redhat" ? "gcc" : (
      fact.os.family == "arch" ? "base-devel" : "gcc"
    )
  )
  ensure = "present"
}
```

## Using Facts in Templates

Facts are available in Go templates via the `.fact` namespace:

**Template file (config.yaml.tpl):**
```yaml
# Application Configuration
# Generated on {{ .fact.hostname }}

system:
  hostname: {{ .fact.hostname }}
  fqdn: {{ .fact.fqdn }}
  arch: {{ .fact.arch }}
  os:
    name: {{ .fact.os.name }}
    family: {{ .fact.os.family }}
    distribution: {{ .fact.os.distribution }}
    version: {{ .fact.os.distribution_version }}

user:
  name: {{ .fact.user.name }}
  home: {{ .fact.user.home }}

# OS-specific settings
{{- if eq .fact.os.family "debian" }}
package_manager: apt
{{- else if eq .fact.os.family "redhat" }}
package_manager: dnf
{{- else if eq .fact.os.family "arch" }}
package_manager: pacman
{{- else if eq .fact.os.family "darwin" }}
package_manager: brew
{{- end }}

# Architecture-specific binary
{{- if eq .fact.arch "amd64" }}
binary_url: https://example.com/app-x86_64.tar.gz
{{- else if eq .fact.arch "arm64" }}
binary_url: https://example.com/app-aarch64.tar.gz
{{- end }}
```

**HCL configuration:**
```hcl
resource "file" "app_config" {
  path    = "${fact.user.home}/.config/myapp/config.yaml"
  content = template("./config.yaml.tpl")
  mode    = "0640"
}
```

## Combining Facts with Variables

Facts work seamlessly with variables:

```hcl
variable "app_name" {
  default = "myapp"
}

resource "directory" "app_config" {
  path = "${fact.user.home}/.config/${var.app_name}"
  mode = "0755"
}

resource "file" "system_info" {
  path    = "${fact.user.home}/.config/${var.app_name}/system.txt"
  content = <<-EOF
    Application: ${var.app_name}
    Running on: ${fact.hostname} (${fact.os.distribution} ${fact.os.distribution_version})
    User: ${fact.user.name}
  EOF
  mode = "0644"

  depends_on = ["directory.app_config"]
}
```

## Example: Cross-Platform Configuration

```hcl
variable "app_name" {
  default = "myapp"
}

# Config directory location varies by OS
resource "directory" "config" {
  path = fact.os.name == "darwin" ? "${fact.user.home}/Library/Application Support/${var.app_name}" : "${fact.user.home}/.config/${var.app_name}"
  mode = "0755"
}

# Install the right package for each OS family
resource "package" "git" {
  name = fact.os.family == "alpine" ? "git" : (
    fact.os.family == "darwin" ? "git" : "git"
  )
  ensure = "present"
}

# Service manager differs between platforms
resource "file" "service_info" {
  path = "${directory.config.path}/service.conf"
  content = join("\n", [
    "# Service configuration",
    format("init_system=%s", fact.os.name == "linux" ? "systemd" : "launchd"),
    format("platform=%s", fact.os.name),
    "",
  ])
  mode = "0644"

  depends_on = ["directory.config"]
}
```
