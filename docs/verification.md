# Host Configuration Verification with hostcfg

## Overview

The `hostcfg verify` command provides native **infrastructure testing** capabilities without requiring separate test specifications. Your HCL configuration serves as both the desired state and the test specification.

## Why Verify?

### Use Cases

1. **Role Testing** - Verify a role configured the host correctly after deployment
2. **Drift Detection** - Detect manual changes to managed resources
3. **Continuous Compliance** - Schedule periodic checks that configuration hasn't drifted
4. **Post-Deployment Validation** - Confirm `apply` succeeded
5. **CI/CD Integration** - Automated verification in deployment pipelines

### Comparison with Other Tools

| Tool | Approach | Test Specs |
|------|----------|------------|
| **hostcfg verify** | Reads HCL config, checks actual state | **None needed** - HCL is the spec |
| Goss | Separate YAML test files | Required |
| InSpec | Separate Ruby test files | Required |
| Serverspec | Separate RSpec tests | Required |
| Ansible --check | Re-runs playbook in check mode | Playbook serves as spec |

**Key Advantage**: Single source of truth. Your configuration IS your test specification.

## Quick Start

```bash
# 1. Apply configuration
hostcfg apply -c webserver.hcl --yes

# 2. Verify everything matches
hostcfg verify -c webserver.hcl

# 3. Simulate drift
sudo chmod 0777 /etc/nginx/nginx.conf
sudo systemctl stop nginx

# 4. Detect drift
hostcfg verify -c webserver.hcl
# Output:
# ✓ package.nginx - verified
# ✗ file.nginx_config - drift detected
#   mode: expected 0644, got 0777
# ✗ service.nginx - drift detected
#   status: expected running, got stopped

# 5. Fix drift
hostcfg apply -c webserver.hcl --yes

# 6. Verify fixed
hostcfg verify -c webserver.hcl
# Output:
# ✓ package.nginx - verified
# ✓ file.nginx_config - verified
# ✓ service.nginx - verified
```

## How It Works

### Verification Process

For each resource, `verify` performs these steps:

1. **Read current state** - Query actual system state (files exist? packages installed? services running?)
2. **Compare with desired** - Match against values specified in HCL
3. **Report mismatches** - Show any attributes that differ

### What Gets Verified

```hcl
resource "file" "nginx_config" {
  path    = "/etc/nginx/nginx.conf"
  content = "worker_processes auto;"
  mode    = "0644"
  owner   = "root"
  group   = "root"
}
```

Verification checks:
- ✓ File exists at `/etc/nginx/nginx.conf`
- ✓ Content matches `"worker_processes auto;"`
- ✓ Mode is `0644`
- ✓ Owner is `root`
- ✓ Group is `root`

### Optional vs Required Attributes

**Only specified attributes are verified:**

```hcl
resource "file" "config" {
  path    = "/etc/app.conf"
  content = "key=value"
  # mode not specified - any mode is acceptable
  # owner not specified - any owner is acceptable
}
```

This verifies **only** that:
- File exists
- Content matches

Mode and owner can be **anything** - they won't cause verification failure.

## Command Reference

### Basic Usage

```bash
# Verify all resources
hostcfg verify

# Verify specific config
hostcfg verify -c path/to/config.hcl

# Verify with variables
hostcfg verify -e environment=production

# Verbose output
hostcfg verify --verbose
```

### Exit Codes

| Code | Meaning |
|------|---------|
| `0` | All resources verified successfully |
| `1` | One or more resources drifted |
| `2` | Error during verification (permissions, network, etc.) |

### Integration with CI/CD

```yaml
# GitHub Actions example
name: Configuration Verification

on:
  schedule:
    - cron: '0 */6 * * *'  # Every 6 hours
  push:
    branches: [main]

jobs:
  verify:
    runs-on: self-hosted
    steps:
      - uses: actions/checkout@v3

      - name: Verify configuration
        run: |
          hostcfg verify

      - name: Alert on drift
        if: failure()
        uses: slack notify action
        with:
          message: "Configuration drift detected!"
```

### Cron/Systemd Timer

```bash
# Add to crontab
0 */6 * * * /usr/local/bin/hostcfg verify || mail -s "Config Drift" admin@example.com
```

## Resource-Specific Verification

### File Resources

```hcl
resource "file" "config" {
  path    = "/etc/app.conf"
  content = "..."
  mode    = "0644"
  owner   = "app"
  group   = "app"
}
```

**Verifies:**
- File exists
- Content matches (SHA256 hash comparison)
- Mode matches (if specified)
- Owner matches (if specified)
- Group matches (if specified)

### Directory Resources

```hcl
resource "directory" "data" {
  path = "/var/lib/app"
  mode = "0750"
}
```

**Verifies:**
- Directory exists
- Mode matches
- Owner/group match (if specified)

### Package Resources

```hcl
resource "package" "nginx" {
  name    = "nginx"
  version = "1.18.0"  # optional
  ensure  = "present"
}
```

**Verifies:**
- Package is installed
- Version matches (if specified)

### Service Resources

```hcl
resource "service" "nginx" {
  name    = "nginx"
  ensure  = "running"
  enabled = true
}
```

**Verifies:**
- Service is running (if ensure="running")
- Service is stopped (if ensure="stopped")
- Service is enabled at boot (if enabled=true)

### Link Resources

```hcl
resource "link" "current" {
  path   = "/app/current"
  target = "/app/releases/v1.2.3"
}
```

**Verifies:**
- Symlink exists
- Target matches

### Download Resources

```hcl
resource "download" "binary" {
  url      = "https://example.com/app.tar.gz"
  dest     = "/usr/local/bin/app"
  checksum = "sha256:abc123..."
}
```

**Verifies:**
- File exists at destination
- Checksum matches
- Mode/owner/group match (if specified)

### User Resources

```hcl
resource "user" "appuser" {
  name   = "appuser"
  shell  = "/bin/bash"
  home   = "/home/appuser"
  groups = ["docker", "sudo"]
}
```

**Verifies:**
- User exists
- Shell matches
- Home directory matches
- Group memberships match

### Group Resources

```hcl
resource "group" "developers" {
  name    = "developers"
  members = ["alice", "bob"]
}
```

**Verifies:**
- Group exists
- Members match

### Exec Resources

```hcl
resource "exec" "init" {
  command = "setup-database.sh"
  creates = "/var/lib/app/initialized"
}
```

**Verifies:**
- Creates file exists (if specified)
- **Does NOT re-run command** (by design)

### Hostname Resources

```hcl
resource "hostname" "main" {
  name = "webserver01"
}
```

**Verifies:**
- System hostname matches

### Stat Resources

```hcl
resource "stat" "check_file" {
  path = "/etc/important.conf"
}
```

**Verifies:**
- Always passes (read-only resource, no desired state to compare)

## Advanced Usage

### Conditional Verification

Resources with `when` conditions are skipped during verification if conditions are false:

```hcl
variable "debug" {
  type    = bool
  default = false
}

resource "file" "debug_log" {
  path    = "/var/log/app-debug.log"
  content = ""

  when = [var.debug]
}
```

```bash
# With debug=false, this resource is skipped
hostcfg verify -e debug=false

# Output:
# ⊘ file.debug_log - skipped (when condition false)
```

### Role Verification

```hcl
role "database" {
  source = "./roles/postgresql"
  variables = {
    version = "14"
  }
}

role "webserver" {
  source = "./roles/nginx"
}
```

```bash
# Verify all roles
hostcfg verify

# All resources from all roles are verified
```

### Ensure="absent" Verification

```hcl
resource "file" "deprecated_config" {
  path   = "/etc/old-app.conf"
  ensure = "absent"
}
```

**Verifies:**
- File does **not** exist
- If file exists, verification fails

## Troubleshooting

### Common Issues

**Issue**: Verification fails immediately after `apply`

```bash
hostcfg apply --yes
hostcfg verify
# ✗ service.nginx - drift detected
```

**Cause**: Service may take time to start, or apply may have failed silently.

**Solution**: Check service logs, add `timeout` to resource, or add delay:

```hcl
resource "service" "nginx" {
  name    = "nginx"
  ensure  = "running"
  timeout = "30s"  # Allow more time for service startup
}
```

**Issue**: Content hash mismatch on file that looks identical

**Cause**: Line ending differences (CRLF vs LF), trailing whitespace

**Solution**: Normalize line endings in HCL:

```hcl
resource "file" "config" {
  content = trimspace(<<-EOF
    # config content
  EOF
  )
}
```

**Issue**: Verification hangs or times out

**Cause**: Network issues (download resources), slow package manager queries

**Solution**: Increase timeout:

```bash
hostcfg verify --timeout 10m
```

### Verbose Mode

```bash
hostcfg verify --verbose
```

Output:
```
  [verbose] Verifying directory.app_root (timeout: 5m)...
  [verbose] Reading current state of directory.app_root (timeout: 5m)...
✓ directory.app_root - verified
  [verbose] Verifying file.config (timeout: 5m)...
✓ file.config - verified
```

## Best Practices

### 1. Test Roles Immediately After Development

```bash
# Development workflow
vim roles/webserver/resources.hcl
hostcfg apply -c test-playbook.hcl --yes
hostcfg verify -c test-playbook.hcl
```

### 2. Schedule Periodic Verification

```bash
# /etc/cron.d/hostcfg-verify
0 */6 * * * root /usr/local/bin/hostcfg verify || /usr/local/bin/alert-drift.sh
```

### 3. Verify Before Deploying New Changes

```bash
# Ensure system is in known state before changes
hostcfg verify || exit 1
hostcfg apply --yes
```

### 4. Use in Immutable Infrastructure

```dockerfile
# Dockerfile
COPY hostcfg.hcl /etc/hostcfg/
RUN hostcfg apply --yes && \
    hostcfg verify && \
    rm -rf /var/cache/*
```

### 5. Version Control Your Configs

```bash
# Track what SHOULD be verified
git commit -m "Add nginx vhost configuration"
git push

# On server
git pull
hostcfg verify  # Should detect drift if manual changes were made
```

## Comparison: verify vs plan

| Command | Purpose | Output |
|---------|---------|--------|
| `hostcfg plan` | Show what **would** change | Resources that **need** changing |
| `hostcfg verify` | Check current state **matches** | Resources that **don't match** |

**Example:**

```bash
# Manually change a file
echo "modified" >> /etc/app.conf

# Plan shows what would change to fix it
hostcfg plan
# Output: ~ file.config
#           ~ content: (changed)

# Verify shows it doesn't match
hostcfg verify
# Output: ✗ file.config - drift detected
#           content: content hash mismatch
```

**When to use each:**

- **verify** - "Is my system configured correctly?"
- **plan** - "What changes will be made?"

## Examples

### Example 1: Simple Drift Detection

```hcl
# config.hcl
resource "file" "motd" {
  path    = "/etc/motd"
  content = "Welcome to production server"
  mode    = "0644"
}
```

```bash
$ hostcfg apply --yes
Applying file.motd...
  Done.

$ hostcfg verify
✓ file.motd - verified

$ echo "Hacked!" > /etc/motd

$ hostcfg verify
✗ file.motd - drift detected
  content: content hash mismatch

$ hostcfg apply --yes
Applying file.motd...
  Done.

$ hostcfg verify
✓ file.motd - verified
```

### Example 2: Multi-Resource Role

See `examples/nginx-verify-demo.hcl` for a complete example.

### Example 3: CI/CD Pipeline

```yaml
# .gitlab-ci.yml
stages:
  - verify
  - deploy

verify-config:
  stage: verify
  script:
    - hostcfg verify
  only:
    - schedules  # Run on schedule to detect drift

deploy:
  stage: deploy
  script:
    - hostcfg apply --yes
    - hostcfg verify  # Ensure apply succeeded
  only:
    - main
```

## Conclusion

The `hostcfg verify` command provides infrastructure testing with **zero additional test code**. Your HCL configuration serves as both the desired state definition and the test specification.

This enables:
- ✅ Role testing without separate test files
- ✅ Drift detection to catch manual changes
- ✅ Continuous compliance monitoring
- ✅ Single source of truth for configuration and tests

Try it today:

```bash
hostcfg verify
```
