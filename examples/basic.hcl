# Basic example demonstrating typed variables and file resources

variable "app_name" {
  type        = string
  default     = "myapp"
  description = "Name of the application"
}

variable "config_dir" {
  type    = string
  default = "/tmp/hostcfg-example"
}

variable "debug" {
  type    = bool
  default = false
}

variable "port" {
  type    = number
  default = 8080
}

# Create the configuration directory
resource "directory" "config_dir" {
  path = var.config_dir
  mode = "0755"
}

# Create a simple configuration file
resource "file" "app_config" {
  path    = "${var.config_dir}/config.json"
  content = <<-EOF
    {
      "name": "${var.app_name}",
      "version": "1.0.0",
      "debug": ${var.debug},
      "port": ${var.port}
    }
  EOF
  mode    = "0644"

  depends_on = ["directory.config_dir"]
}

# Create a README file
resource "file" "readme" {
  path    = "${var.config_dir}/README.txt"
  content = "This directory contains configuration for ${var.app_name}.\n"
  mode    = "0644"

  depends_on = ["directory.config_dir"]
}
