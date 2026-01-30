# Advanced Templating Example
#
# This example demonstrates the powerful Sprig template functions available
# in hostcfg. Templates have access to all Sprig functions, providing
# Helm-style templating capabilities.
#
# See: https://masterminds.github.io/sprig/

# --- Variables ---

variable "app_name" {
  default     = "My Application"
  description = "Application display name"
}

variable "environment" {
  default     = "development"
  description = "Deployment environment (development, staging, production)"
}

variable "domain" {
  default     = "example.com"
  description = "Application domain"
}

variable "server_port" {
  default     = "8080"
  description = "Server listen port"
}

variable "tls_enabled" {
  default     = false
  description = "Enable TLS/SSL"
}

variable "api_key" {
  default     = "secret-api-key-12345"
  description = "API key for external services"
}

variable "log_level" {
  default     = "info"
  description = "Logging level"
}

variable "log_to_file" {
  default     = true
  description = "Write logs to file"
}

variable "config_path" {
  default     = "/opt/myapp"
  description = "Base configuration path"
}

variable "organization" {
  default     = "ACME Corp"
  description = "Organization name"
}

variable "banner" {
  default     = "Welcome to My Application\nPowered by hostcfg"
  description = "Application banner message"
}

# Complex variables: lists and maps

variable "features" {
  default     = ["metrics", "tracing", "health-checks"]
  description = "Enabled feature flags"
}

variable "users" {
  default     = ["admin", "operator", "viewer"]
  description = "Default users to create"
}

variable "database" {
  default = {
    host     = "localhost"
    port     = "5432"
    name     = "myapp_db"
    pool_size = "10"
  }
  description = "Database connection settings"
}

variable "metadata" {
  default = {
    version = "1.0.0"
    team    = "platform"
    tier    = "backend"
  }
  description = "Application metadata"
}

variable "labels" {
  default = {
    app         = "myapp"
    component   = "api"
    managed_by  = "hostcfg"
  }
  description = "Resource labels"
}

# --- Resources ---

# Create the configuration directory
resource "directory" "config" {
  path = var.config_path
  mode = "0755"
}

# Generate the application configuration using the template
# The template demonstrates:
# - String functions: upper, lower, title, replace, trim, indent
# - Default values and ternary operators
# - Conditionals: if/else, and, or, eq, ne
# - Iteration: range over lists and maps
# - Encoding: base64, sha256, JSON, YAML
# - List operations: append, uniq
# - Dict operations: dict, merge
# - Date formatting: now, date
resource "file" "app_config" {
  path    = "${var.config_path}/config.yaml"
  content = template("./files/app-config.yaml.tpl")
  mode    = "0640"

  depends_on = ["directory.config"]
}

# Example: Inline template using heredoc with Sprig functions
# (Note: HCL interpolation happens first, then Go template execution)
resource "file" "env_file" {
  path    = "${var.config_path}/.env"
  content = <<-EOF
    # Environment configuration
    APP_NAME=${var.app_name}
    APP_ENV=${var.environment}
    APP_PORT=${var.server_port}
    APP_DOMAIN=${var.domain}

    # Database (from variable map)
    DB_HOST=${var.database.host}
    DB_PORT=${var.database.port}
    DB_NAME=${var.database.name}

    # Features (joined from list)
    ENABLED_FEATURES=${join(",", var.features)}
  EOF
  mode = "0600"

  depends_on = ["directory.config"]
}

# Another template example: nginx configuration
resource "file" "nginx_upstream" {
  path    = "${var.config_path}/nginx-upstream.conf"
  content = template("./files/nginx-upstream.conf.tpl")
  mode    = "0644"

  depends_on = ["directory.config"]
}
