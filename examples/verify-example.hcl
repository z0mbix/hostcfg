# Example configuration for testing the verify command

variable "app_name" {
  type    = string
  default = "testapp"
}

# Create a test directory
resource "directory" "test_dir" {
  description = "Test directory for verification"
  path        = "/tmp/hostcfg-verify-test"
  mode        = "0755"
}

# Create a test file
resource "file" "test_config" {
  description = "Test config file"
  path        = "/tmp/hostcfg-verify-test/config.txt"
  content     = <<-EOF
    # ${var.app_name} Configuration
    # This file is managed by hostcfg
    app_name = ${var.app_name}
    version = 1.0
  EOF
  mode        = "0644"

  depends_on = ["directory.test_dir"]
}

# Create another file with specific permissions
resource "file" "test_data" {
  description = "Test data file"
  path        = "/tmp/hostcfg-verify-test/data.json"
  content     = <<-EOF
    {
      "app": "${var.app_name}",
      "timestamp": "2024-01-01"
    }
  EOF
  mode        = "0600"

  depends_on = ["directory.test_dir"]
}

# Create a symbolic link
resource "link" "test_link" {
  description = "Symbolic link to config"
  path        = "/tmp/hostcfg-verify-test/current-config"
  target      = "/tmp/hostcfg-verify-test/config.txt"

  depends_on = ["file.test_config"]
}

# Test exec resource (creates a marker file)
resource "exec" "test_init" {
  description = "Initialize test environment"
  command     = "touch /tmp/hostcfg-verify-test/initialized && echo 'Initialized' > /tmp/hostcfg-verify-test/initialized"
  creates     = "/tmp/hostcfg-verify-test/initialized"

  depends_on = ["directory.test_dir"]
}
