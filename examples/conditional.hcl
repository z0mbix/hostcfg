# Example: Conditional Resource Execution
#
# The `when` attribute accepts a list of boolean expressions.
# All conditions must be true for the resource to execute;
# otherwise it's skipped.

# Check if kubectl already exists
resource "stat" "kubectl_bin" {
  path = "/usr/local/bin/kubectl"
}

# Only download kubectl if it doesn't exist and we're on amd64
resource "download" "kubectl" {
  when = [
    !stat.kubectl_bin.exists,
    fact.arch == "amd64",
  ]

  url  = "https://dl.k8s.io/release/v1.28.0/bin/linux/amd64/kubectl"
  dest = "/usr/local/bin/kubectl"
  mode = "0755"
}

# Check if a config directory exists
resource "stat" "config_dir" {
  path = "/etc/myapp"
}

# Create config directory only if it doesn't exist
resource "directory" "config_dir" {
  when = [!stat.config_dir.exists]

  path = "/etc/myapp"
  mode = "0755"
}

# This config file depends on the directory, and will be skipped
# if the directory creation is skipped (cascade skip)
resource "file" "config" {
  when = [!stat.config_dir.exists]

  path       = "/etc/myapp/app.conf"
  content    = "# App configuration\nport = 8080\n"
  depends_on = ["directory.config_dir"]
}

# Example: Platform-specific resources (fact.os.name is "linux" or "darwin")
resource "file" "linux_specific" {
  when = [fact.os.name == "linux"]

  path    = "/tmp/platform.txt"
  content = "Running on Linux (${fact.os.distribution})\n"
}

# Example: Multiple conditions (all must be true)
# fact.os.family is "debian", "redhat", "arch", "darwin", etc.
resource "file" "debian_only" {
  when = [
    fact.os.name == "linux",
    fact.os.family == "debian",
    fact.arch == "amd64",
  ]

  path    = "/tmp/debian-amd64.txt"
  content = "Running on Debian/Ubuntu amd64\n"
}
