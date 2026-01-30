variable "version" {
  default = "1.2.3"
}

resource "directory" "config" {
  path = "/tmp/myapp"
  mode = "0755"
}

resource "file" "config" {
  path    = "${directory.config.path}/config.json"
  content = <<-EOF
    {"name": "myapp", "version": "${var.version}"}
  EOF
}
