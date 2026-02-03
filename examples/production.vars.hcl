# Production environment variables
# Use with: hostcfg apply --var-file production.vars.hcl

app_name    = "myapp"
config_dir  = "/etc/myapp"
environment = "production"
debug       = false
log_level   = "warn"

# Server configuration
server_config = {
  workers = 8
  timeout = 60
}

# Allowed hosts
allowed_hosts = ["web1.example.com", "web2.example.com"]
