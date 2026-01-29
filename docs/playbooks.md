# Playbooks

Hostcfg supports Ansible-style playbook patterns through roles and role dependencies. This allows you to compose multiple reusable roles and apply them to hosts in a deterministic order.

## Basic Usage

Define multiple roles in your host configuration with `depends_on` to control execution order:

```hcl
# hosts/webserver/hostcfg.hcl

role "base" {
  source = "../../roles/base"
}

role "redis" {
  source     = "../../roles/redis"
  depends_on = ["role.base"]

  variables = {
    port      = 6379
    maxmemory = "512mb"
  }
}

role "webapp" {
  source     = "../../roles/webapp"
  depends_on = ["role.redis"]

  variables = {
    redis_host = "localhost"
    redis_port = 6379
  }
}
```

## How Role Dependencies Work

When you specify `depends_on = ["role.redis"]`, hostcfg expands this to include **all resources** in the redis role. This ensures the entire role completes before dependent roles start.

For example, if the redis role contains:
- `package.redis_redis`
- `file.redis_config`
- `service.redis_redis`

Then `depends_on = ["role.redis"]` is equivalent to:
```hcl
depends_on = [
  "package.redis_redis",
  "file.redis_config",
  "service.redis_redis"
]
```

## Execution Order

Resources are applied in topological order based on dependencies:

1. Resources with no dependencies run first
2. Resources wait for all their dependencies to complete
3. Role-level dependencies ensure entire roles complete before dependent roles start

Example execution order for the configuration above:

```
1. base role resources (no dependencies)
   ├── package.base_essentials
   ├── file.base_sshd_config
   └── ...

2. redis role resources (depends on role.base)
   ├── package.redis_redis
   ├── file.redis_config
   └── service.redis_redis

3. webapp role resources (depends on role.redis)
   ├── directory.webapp_root
   ├── file.webapp_config
   └── service.webapp_app
```

## Variable Precedence

Variables are resolved with the following precedence (highest to lowest):

1. **CLI variables** (`-e port=6380`) - override everything
2. **Role instantiation variables** (`variables = { ... }`)
3. **Role defaults** (`roles/redis/variables.hcl`)

```hcl
# roles/redis/variables.hcl
variable "port" {
  default = 6379
}

variable "maxmemory" {
  default = "256mb"
}
```

```hcl
# hosts/webserver/hostcfg.hcl
role "redis" {
  source = "../../roles/redis"

  variables = {
    maxmemory = "1gb"  # Overrides default
  }
}
```

```bash
# CLI override takes highest precedence
hostcfg apply -c ./hosts/webserver -e port=6380
```

## Directory Structure

A typical multi-host setup with shared roles:

```
.
├── roles/
│   ├── base/
│   │   ├── variables.hcl
│   │   ├── resources.hcl
│   │   └── files/
│   │       └── sshd_config.tpl
│   ├── redis/
│   │   ├── variables.hcl
│   │   ├── resources.hcl
│   │   └── files/
│   │       └── redis.conf.tpl
│   └── webapp/
│       ├── variables.hcl
│       ├── resources.hcl
│       └── files/
│           └── config.tpl
├── hosts/
│   ├── webserver/
│   │   ├── hostcfg.hcl
│   │   └── variables.hcl
│   ├── database/
│   │   ├── hostcfg.hcl
│   │   └── variables.hcl
│   └── loadbalancer/
│       ├── hostcfg.hcl
│       └── variables.hcl
```

## Example: Multi-Role Web Server

```hcl
# hosts/webserver/variables.hcl
variable "environment" {
  default = "production"
}

variable "app_port" {
  default = 8080
}
```

```hcl
# hosts/webserver/hostcfg.hcl

# Base system configuration
role "base" {
  source = "../../roles/base"

  variables = {
    environment = var.environment
  }
}

# Redis for session storage
role "redis" {
  source     = "../../roles/redis"
  depends_on = ["role.base"]

  variables = {
    port      = 6379
    maxmemory = "256mb"
    bind      = "127.0.0.1"
  }
}

# Application server
role "app" {
  source     = "../../roles/app"
  depends_on = ["role.redis"]

  variables = {
    port        = var.app_port
    environment = var.environment
    redis_host  = "127.0.0.1"
    redis_port  = 6379
  }
}

# Nginx reverse proxy
role "nginx" {
  source     = "../../roles/nginx"
  depends_on = ["role.app"]

  variables = {
    upstream_port = var.app_port
    server_name   = "example.com"
  }
}

# Monitoring agent
role "monitoring" {
  source     = "../../roles/monitoring"
  depends_on = ["role.nginx"]

  variables = {
    environment = var.environment
  }
}
```

Apply the configuration:

```bash
# Plan changes
hostcfg plan -c ./hosts/webserver

# Apply changes
hostcfg apply -c ./hosts/webserver

# Apply with environment override
hostcfg apply -c ./hosts/webserver -e environment=staging
```

## Tips

- **Keep roles focused**: Each role should handle one concern (e.g., redis, nginx, app)
- **Use role defaults**: Define sensible defaults in `variables.hcl`
- **Document variables**: Add descriptions to variables for clarity
- **Test incrementally**: Apply roles one at a time during development
- **Use `recursive = true`**: For directory resources that may need parent directories created
