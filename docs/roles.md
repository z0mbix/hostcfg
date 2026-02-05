# Roles

Roles are reusable configuration modules that contain default variables, resources, and template files.

## Role Directory Structure

```
roles/
  redis/
    variables.hcl     # Default variables (lowest precedence)
    resources.hcl     # Role's HCL resources
    files/
      redis.conf.tpl  # Template files
      file.txt        # Regular files
```

## Using Roles

```hcl
variable "redis_port" {
  type    = number
  default = 6379
}

role "redis" {
  source = "./roles/redis"

  variables = {
    port    = var.redis_port
    version = "6.0.1"
  }
}

role "webapp" {
  source     = "./roles/webapp"
  depends_on = ["role.redis"]  # Role-level dependency
}
```

## Resource Namespacing

Role resources are automatically prefixed with the role name to avoid conflicts:

- `file.config` in role `redis` → `file.redis_config`
- `service.redis` in role `redis` → `service.redis_redis`

## Variable Precedence

Variables are resolved with the following precedence (highest to lowest):

1. **CLI variables** (`-e port=6380`)
2. **Role instantiation variables** (`variables = { ... }`)
3. **Role defaults** (`roles/redis/variables.hcl`)

## Path Resolution

Template paths in roles resolve relative to the role directory:

- `template("files/redis.conf.tpl")` → `roles/redis/files/redis.conf.tpl`

## Dependencies

### Intra-role Dependencies

Dependencies within a role are auto-prefixed:

```hcl
# In roles/redis/resources.hcl
resource "file" "config" {
  depends_on = ["package.redis"]  # Becomes package.redis_redis
}
```

### Role-level Dependencies

Use `depends_on = ["role.xxx"]` to depend on all resources in another role:

```hcl
role "webapp" {
  source     = "./roles/webapp"
  depends_on = ["role.redis"]  # Expands to all redis role resources
}
```

### Cross-reference

Main config can reference role resources using their prefixed names:

```hcl
resource "file" "app_config" {
  content = "redis_config=${file.redis_config.path}"
}
```

## Example Role

**`roles/redis/variables.hcl`:**
```hcl
variable "port" {
  type    = number
  default = 6379
}

variable "maxmemory" {
  type    = string
  default = "256mb"
}

variable "databases" {
  type    = number
  default = 16
}
```

**`roles/redis/resources.hcl`:**
```hcl
resource "package" "redis" {
  name   = "redis-server"
  ensure = "present"
}

resource "file" "config" {
  path       = "/etc/redis/redis.conf"
  content    = template("files/redis.conf.tpl")
  mode       = "0640"
  depends_on = ["package.redis"]
}

resource "service" "redis" {
  name       = "redis-server"
  ensure     = "running"
  enabled    = true
  depends_on = ["file.config"]
}
```

**`roles/redis/files/redis.conf.tpl`:**
```
port {{ .var.port }}
maxmemory {{ .var.maxmemory }}
bind 127.0.0.1
```

See [Playbooks](playbooks.md) for using multiple roles together.
