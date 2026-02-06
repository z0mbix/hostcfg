# System Facts Example
#
# This example demonstrates using system facts in HCL expressions
# and templates. Facts provide Ansible-style system information.
#
# Available facts:
#   fact.os.name                - OS name (linux, darwin)
#   fact.os.family              - OS family (debian, redhat, arch, darwin)
#   fact.os.distribution        - Distribution name (Ubuntu, Fedora)
#   fact.os.distribution_version - Version (22.04, 39)
#   fact.arch                   - Architecture (amd64, arm64)
#   fact.hostname               - System hostname
#   fact.fqdn                   - Fully qualified domain name
#   fact.user.name              - Current username
#   fact.user.home              - Home directory
#   fact.user.uid               - User ID
#   fact.user.gid               - Group ID

# --- Variables ---

variable "app_name" {
  default     = "myapp"
  description = "Application name"
}

# --- Resources ---

# Create app config directory in user's home
resource "directory" "app_config" {
  path = "${fact.user.home}/.config/${var.app_name}"
  mode = "0755"
}

# Install the appropriate editor package based on OS family
resource "package" "editor" {
  name = fact.os.family == "debian" ? "vim" : (
    fact.os.family == "redhat" ? "vim-enhanced" : (
      fact.os.family == "arch" ? "vim" : "vim"
    )
  )
  ensure = "present"
}

# Install OS-specific packages
resource "package" "build_essential" {
  name = fact.os.family == "debian" ? "build-essential" : (
    fact.os.family == "redhat" ? "gcc" : (
      fact.os.family == "arch" ? "base-devel" : "gcc"
    )
  )
  ensure = "present"
}

# Create a system info file using facts
resource "file" "system_info" {
  path    = "${directory.app_config.path}/system-info.txt"
  content = <<-EOF
    System Information
    ==================
    Hostname: ${fact.hostname}
    FQDN: ${fact.fqdn}

    Operating System:
      Name: ${fact.os.name}
      Family: ${fact.os.family}
      Distribution: ${fact.os.distribution}
      Version: ${fact.os.distribution_version}

    Architecture: ${fact.arch}

    User:
      Username: ${fact.user.name}
      Home: ${fact.user.home}
      UID: ${fact.user.uid}
      GID: ${fact.user.gid}
  EOF
  mode    = "0644"
}

# Use facts in a template file
resource "file" "app_config" {
  path    = "${directory.app_config.path}/config.yaml"
  content = template("./files/facts-config.yaml.tpl")
  mode    = "0640"
}

# Use ternary operator for simple OS-specific values
resource "file" "os_specific" {
  path = "${directory.app_config.path}/os-specific.conf"
  content = join("\n", [
    "# OS-specific configuration",
    "# Generated for ${fact.os.distribution} (${fact.os.family})",
    "",
    format("package_manager=%s", fact.os.family == "debian" ? "apt" : (fact.os.family == "redhat" ? "dnf" : (fact.os.family == "arch" ? "pacman" : "unknown"))),
    format("service_manager=%s", fact.os.name == "linux" ? "systemd" : "launchd"),
    format("os_family=%s", fact.os.family),
    "",
  ])
  mode = "0644"
}
