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
