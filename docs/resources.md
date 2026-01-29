# Resources

## file

Manages files with content, ownership, and permissions.

```hcl
resource "file" "example" {
  path    = "/etc/myapp/config.conf"
  content = "key = value\n"
  owner   = "root"
  group   = "root"
  mode    = "0644"
  ensure  = "present"  # or "absent"
}
```

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | yes | Absolute path to the file |
| `content` | string | yes* | File content (mutually exclusive with `source`) |
| `source` | string | yes* | Path to source file to copy (mutually exclusive with `content`) |
| `owner` | string | no | File owner username |
| `group` | string | no | File group name |
| `mode` | string | no | File permissions in octal (default: `0644`) |
| `ensure` | string | no | `present` (default) or `absent` |

**Idempotency**: Checks SHA256 hash of content and file stat for ownership/permissions.

## directory

Manages directories with ownership and permissions.

```hcl
resource "directory" "example" {
  path      = "/var/lib/myapp"
  owner     = "appuser"
  group     = "appgroup"
  mode      = "0750"
  recursive = true
  ensure    = "present"  # or "absent"
}
```

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | yes | Absolute path to the directory |
| `owner` | string | no | Directory owner username |
| `group` | string | no | Directory group name |
| `mode` | string | no | Directory permissions in octal (default: `0755`) |
| `recursive` | bool | no | Create parent directories / apply ownership recursively |
| `ensure` | string | no | `present` (default) or `absent` |

**Idempotency**: Checks directory existence and stat for ownership/permissions.

## exec

Executes commands with conditional guards for idempotency.

```hcl
resource "exec" "example" {
  command = "make install"
  dir     = "/opt/myapp"
  user    = "root"
  creates = "/usr/local/bin/myapp"  # Only run if this file doesn't exist
  only_if = "test -f Makefile"      # Only run if this command succeeds
  unless  = "which myapp"           # Don't run if this command succeeds
}
```

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | string | yes | Command to execute (run via `sh -c`) |
| `dir` | string | no | Working directory for the command |
| `user` | string | no | User to run the command as |
| `creates` | string | no | Path to a file; command only runs if file doesn't exist |
| `only_if` | string | no | Guard command; main command only runs if this succeeds |
| `unless` | string | no | Guard command; main command only runs if this fails |

**Idempotency**: Use `creates`, `only_if`, or `unless` to make exec resources idempotent.

## package

Manages system packages with automatic package manager detection.

```hcl
resource "package" "nginx" {
  name    = "nginx"
  version = "1.18.0"  # Optional: specific version
  ensure  = "present" # or "absent"
}
```

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | yes | Package name |
| `version` | string | no | Specific version to install |
| `ensure` | string | no | `present` (default) or `absent` |

**Supported package managers** (auto-detected):

Linux:
- `apt` (Debian, Ubuntu)
- `dnf` (Fedora, RHEL 8+)
- `yum` (RHEL 7, CentOS)
- `pacman` (Arch Linux)

BSD:
- `pkg` (FreeBSD)
- `pkg_add` (OpenBSD)
- `pkgin` (NetBSD, preferred)
- `pkg_add` (NetBSD, fallback)

**Idempotency**: Queries package manager to check if package is installed and at correct version.

## service

Manages systemd services.

```hcl
resource "service" "nginx" {
  name    = "nginx"
  ensure  = "running"  # or "stopped"
  enabled = true       # Start on boot

  depends_on = ["package.nginx"]
}
```

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | yes | Service name (without `.service` suffix) |
| `ensure` | string | no | `running` or `stopped` |
| `enabled` | bool | no | Whether the service starts on boot |

**Idempotency**: Uses `systemctl is-active` and `systemctl is-enabled` to check current state.

## cron

Manages cron jobs.

```hcl
resource "cron" "backup" {
  command  = "/usr/local/bin/backup.sh >> /var/log/backup.log 2>&1"
  schedule = "0 2 * * *"  # Daily at 2am
  user     = "root"
  ensure   = "present"    # or "absent"
}
```

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | string | yes | Command to execute |
| `schedule` | string | yes | Cron schedule expression (5 fields: min hour dom mon dow) |
| `user` | string | no | User whose crontab to modify (default: current user) |
| `ensure` | string | no | `present` (default) or `absent` |

**Idempotency**: Parses user's crontab and looks for managed entries by marker comment.

## hostname

Manages the system hostname.

```hcl
resource "hostname" "main" {
  name = "webserver01.example.com"
}
```

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | yes | Desired hostname |

**Idempotency**: Reads `/etc/hostname` and compares with desired value.

## user

Manages system users.

```hcl
resource "user" "appuser" {
  name        = "appuser"
  comment     = "Application User"
  home        = "/home/appuser"
  shell       = "/bin/bash"
  groups      = ["appgroup", "docker"]
  create_home = true

  depends_on = ["group.appgroup"]
}
```

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | yes | Username |
| `uid` | string | no | User ID |
| `gid` | string | no | Primary group ID |
| `groups` | list | no | Supplementary groups |
| `home` | string | no | Home directory path |
| `shell` | string | no | Login shell |
| `comment` | string | no | GECOS field (full name, etc.) |
| `system` | bool | no | Create as system user |
| `create_home` | bool | no | Create home directory |
| `ensure` | string | no | `present` (default) or `absent` |

**Idempotency**: Reads `/etc/passwd` to check if user exists and compare attributes.

## group

Manages system groups.

```hcl
resource "group" "developers" {
  name    = "developers"
  members = ["alice", "bob"]
}
```

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | yes | Group name |
| `gid` | string | no | Group ID |
| `members` | list | no | Group members (usernames) |
| `system` | bool | no | Create as system group |
| `ensure` | string | no | `present` (default) or `absent` |

**Idempotency**: Reads `/etc/group` to check if group exists and compare members.

## link

Manages symbolic links.

```hcl
resource "link" "current" {
  path   = "/opt/myapp/current"
  target = "/opt/myapp/releases/v1.2.3"
}

resource "link" "config" {
  path   = "/etc/nginx/sites-enabled/myapp"
  target = "/etc/nginx/sites-available/myapp"
  force  = true  # Replace existing file if present
}
```

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | yes | Path where the symlink will be created |
| `target` | string | yes | Target path the symlink points to |
| `force` | bool | no | Replace existing file/directory with symlink |
| `ensure` | string | no | `present` (default) or `absent` |

**Idempotency**: Uses `os.Readlink` to check current symlink target.

## Resource References

Resources can reference attributes of other resources using `resource_type.resource_name.attribute`. Dependencies are automatically inferred:

```hcl
resource "directory" "app" {
  path = "/opt/myapp"
}

resource "file" "config" {
  path    = "${directory.app.path}/config.json"  # Auto-depends on directory.app
  content = "{}"
}
```

### Available Attributes by Resource Type

| Resource | Available Attributes |
|----------|---------------------|
| `file` | `path`, `content`, `mode`, `owner`, `group` |
| `directory` | `path`, `mode`, `owner`, `group` |
| `exec` | `command`, `creates`, `dir` |
| `hostname` | `name` |
| `cron` | `command`, `schedule`, `user` |
| `package` | `name`, `version` |
| `service` | `name` |
| `user` | `name`, `uid`, `gid`, `home`, `shell` |
| `group` | `name`, `gid` |
| `link` | `path`, `target` |
