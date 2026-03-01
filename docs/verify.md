# Verification Command

## Overview

The `verify` command validates that the actual system state matches the desired state defined in your hostcfg configuration. Unlike `plan` which shows what would change, and `apply` which makes changes, `verify` confirms resources are already correctly configured.

## Usage

```bash
# Verify all resources match desired state
hostcfg verify

# Verify specific config file
hostcfg verify -c webserver.hcl

# Verify with variables
hostcfg verify -e environment=production

# Verbose output
hostcfg verify --verbose
```

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | All resources verified successfully |
| `1` | One or more resources have drifted from desired state |
| `2` | Error during verification (permissions, network, etc.) |

## How It Works

Verification performs these steps for each resource:

1. **Read current state** - Check actual system state (files, packages, services, etc.)
2. **Compare with desired** - Match against HCL configuration
3. **Report mismatches** - Show any drift from desired state

## Example Output

### All Resources Verified

```
✓ package.nginx - verified
✓ service.nginx - verified
✓ file.nginx_config - verified

Verification Summary:
  3 resource(s) verified
```

### Drift Detected

```
✓ package.nginx - verified
✗ service.nginx - drift detected
  status: expected running, got stopped
✗ file.nginx_config - drift detected
  mode: expected 0644, got 0666

Verification Summary:
  1 resource(s) verified
  2 resource(s) drifted
```

### With Skipped Resources

```
✓ package.nginx - verified
⊘ file.debug_log - skipped (when condition false)
✓ service.nginx - verified

Verification Summary:
  2 resource(s) verified
  1 resource(s) skipped
```

## Resource Verification

Each resource type verifies different attributes:

### file
- ✓ File exists
- ✓ Content matches (SHA256 hash)
- ✓ Mode/permissions match (if specified)
- ✓ Owner matches (if specified)
- ✓ Group matches (if specified)

### directory
- ✓ Directory exists
- ✓ Mode/permissions match (if specified)
- ✓ Owner matches (if specified)
- ✓ Group matches (if specified)

### package
- ✓ Package is installed
- ✓ Version matches (if specified)

### service
- ✓ Service state (running/stopped)
- ✓ Boot enabled state (if specified)

### user
- ✓ User exists
- ✓ UID matches (if specified)
- ✓ GID matches (if specified)
- ✓ Home directory matches (if specified)
- ✓ Shell matches (if specified)
- ✓ Groups match (if specified)

### group
- ✓ Group exists
- ✓ GID matches (if specified)
- ✓ Members match (if specified)

### link
- ✓ Symlink exists
- ✓ Target matches

### download
- ✓ File exists at destination
- ✓ Checksum matches (if specified)
- ✓ Mode/owner/group match (if specified)

### exec
- ✓ Creates file exists (if specified)
- ⚠️ Command is **not** re-executed (by design)

### hostname
- ✓ Hostname matches

### cron
- ✓ Cron job exists
- ✓ Schedule matches
- ✓ Command matches
- ✓ User matches (if specified)

### stat
- ℹ️ Read-only resource, always passes (no desired state)

## Verification Semantics

### ensure="present"
Resource must exist and match all specified attributes.

### ensure="absent"
Resource must **not** exist on the system.

### Optional Attributes
If an attribute is not specified in the HCL configuration, any value is acceptable.

**Example:**
```hcl
resource "file" "config" {
  path    = "/etc/app.conf"
  content = "key=value"
  # mode not specified - any mode is acceptable
}
```

This verifies:
- ✓ File exists
- ✓ Content matches

But **does not** verify mode (any mode is OK).

## Use Cases

### 1. Role Testing

Test that a role configured the host correctly:

```bash
# Apply role
hostcfg apply -c playbook.hcl --yes

# Verify it worked
hostcfg verify -c playbook.hcl
```

### 2. Drift Detection

Detect manual changes to managed resources:

```bash
# Schedule periodic checks
0 */6 * * * /usr/local/bin/hostcfg verify || mail -s "Drift detected" admin@example.com
```

### 3. Post-Apply Validation

Confirm that `apply` succeeded:

```bash
hostcfg apply --yes && hostcfg verify
```

### 4. CI/CD Integration

```yaml
# .github/workflows/verify.yml
name: Configuration Verification

on:
  schedule:
    - cron: '0 */6 * * *'

jobs:
  verify:
    runs-on: self-hosted
    steps:
      - name: Verify configuration
        run: hostcfg verify
```

## Comparison with Other Commands

| Command | Purpose | Makes Changes |
|---------|---------|---------------|
| `plan` | Show what **would** change | No |
| `apply` | **Make** changes | Yes |
| `verify` | Check current state **matches** desired | No |

**When to use each:**

- `plan` - "What changes will be made?"
- `apply` - "Make those changes"
- `verify` - "Is my system configured correctly?"

## Comparison with Other Tools

| Tool | Test Specification |
|------|-------------------|
| **hostcfg verify** | HCL config (no separate tests needed) |
| Goss | Separate YAML test files |
| InSpec | Separate Ruby control files |
| Serverspec | Separate RSpec test files |
| Molecule | Uses InSpec/Testinfra for verification |

**Key Advantage**: Single source of truth - your HCL configuration serves as both the desired state and the test specification.

## See Also

- [VERIFICATION.md](../VERIFICATION.md) - Comprehensive verification guide
- [Examples](../examples/) - Example configurations with verification
- [CLI Reference](cli.md) - All available commands
