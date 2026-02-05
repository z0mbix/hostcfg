# Complete example demonstrating all resource types with typed variables

variable "hostname" {
  type    = string
  default = "webserver01"
}

variable "app_user" {
  type    = string
  default = "appuser"
}

variable "app_dir" {
  type    = string
  default = "/opt/myapp"
}

variable "app_port" {
  type    = number
  default = 8080
}

variable "workers" {
  type    = number
  default = 4
}

variable "debug" {
  type    = bool
  default = false
}

# Set the hostname
resource "hostname" "main" {
  name = var.hostname
}

# Install required packages
resource "package" "git" {
  name   = "git"
  ensure = "present"
}

resource "package" "curl" {
  name   = "curl"
  ensure = "present"
}

resource "package" "nginx" {
  name   = "nginx"
  ensure = "present"
}

# Create application directory structure
resource "directory" "app_root" {
  path = var.app_dir
  mode = "0755"
}

resource "directory" "app_config" {
  path = "${var.app_dir}/config"
  mode = "0750"

  depends_on = ["directory.app_root"]
}

resource "directory" "app_logs" {
  path = "${var.app_dir}/logs"
  mode = "0755"

  depends_on = ["directory.app_root"]
}

resource "directory" "app_data" {
  path = "${var.app_dir}/data"
  mode = "0750"

  depends_on = ["directory.app_root"]
}

# Create application configuration
resource "file" "app_config" {
  path    = "${var.app_dir}/config/app.conf"
  content = <<-EOF
    # Application Configuration
    # Managed by hostcfg - do not edit manually

    [general]
    hostname = ${var.hostname}
    log_dir = ${var.app_dir}/logs
    data_dir = ${var.app_dir}/data

    [server]
    port = ${var.app_port}
    workers = ${var.workers}
    timeout = 30
    debug = ${var.debug}

    [database]
    host = localhost
    port = 5432
    name = myapp_db
  EOF
  mode = "0640"

  depends_on = ["directory.app_config"]
}

# Create environment file
resource "file" "env_file" {
  path    = "${var.app_dir}/.env"
  content = <<-EOF
    APP_ENV=production
    APP_DEBUG=false
    APP_LOG_LEVEL=info
    HOSTNAME=${var.hostname}
  EOF
  mode = "0600"

  depends_on = ["directory.app_root"]
}

# Create a startup script
resource "file" "startup_script" {
  path    = "${var.app_dir}/start.sh"
  content = <<-EOF
    #!/bin/bash
    set -e

    cd ${var.app_dir}
    source .env

    echo "Starting application on $HOSTNAME..."
    exec ./bin/myapp --config config/app.conf
  EOF
  mode = "0755"

  depends_on = ["file.app_config", "file.env_file"]
}

# Run an initialization command (only if marker file doesn't exist)
resource "exec" "init_app" {
  command = "touch ${var.app_dir}/.initialized && echo 'Application initialized' >> ${var.app_dir}/logs/init.log"
  creates = "${var.app_dir}/.initialized"
  dir     = var.app_dir

  depends_on = ["directory.app_logs"]
}

# Set up nginx as reverse proxy
resource "file" "nginx_proxy" {
  path    = "/etc/nginx/sites-available/myapp"
  content = <<-EOF
    upstream myapp {
        server 127.0.0.1:${var.app_port};
    }

    server {
        listen 80;
        server_name ${var.hostname};

        location / {
            proxy_pass http://myapp;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        }
    }
  EOF
  mode = "0644"

  depends_on = ["package.nginx"]
}

# Enable nginx site
resource "exec" "enable_nginx_site" {
  command = "ln -sf /etc/nginx/sites-available/myapp /etc/nginx/sites-enabled/myapp"
  creates = "/etc/nginx/sites-enabled/myapp"

  depends_on = ["file.nginx_proxy"]
}

# Ensure nginx is running
resource "service" "nginx" {
  name    = "nginx"
  ensure  = "running"
  enabled = true

  depends_on = ["exec.enable_nginx_site"]
}

# Set up cron job for log rotation
resource "cron" "log_rotation" {
  command  = "find ${var.app_dir}/logs -name '*.log' -mtime +7 -exec gzip {} \\;"
  schedule = "0 4 * * *"
  user     = "root"

  depends_on = ["directory.app_logs"]
}

# Health check cron (every 5 minutes)
resource "cron" "health_check" {
  command  = "curl -s http://localhost:${var.app_port}/health || systemctl restart myapp"
  schedule = "*/5 * * * *"
  user     = "root"

  depends_on = ["package.curl"]
}
