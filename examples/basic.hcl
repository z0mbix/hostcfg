# Basic example demonstrating variables and file resources

variable "app_name" {
  default = "myapp"
}

variable "config_dir" {
  default = "/tmp/hostcfg-example"
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
      "debug": false
    }
  EOF
  mode = "0644"

  depends_on = ["directory.config_dir"]
}

# Create a README file
resource "file" "readme" {
  path    = "${var.config_dir}/README.txt"
  content = "This directory contains configuration for ${var.app_name}.\n"
  mode    = "0644"

  depends_on = ["directory.config_dir"]
}
