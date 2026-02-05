# CLI Reference

## Commands

### plan

Show what changes would be made without applying them.

```bash
hostcfg plan
hostcfg plan -c /path/to/config.hcl
hostcfg plan -e app_name=customapp
```

### apply

Apply changes to bring the system to the desired state.

```bash
hostcfg apply                    # Interactive confirmation
hostcfg apply -y                 # Skip confirmation
hostcfg apply --yes              # Skip confirmation
hostcfg apply --auto-approve     # Skip confirmation (alias for --yes)
hostcfg apply --dry-run          # Same as plan
```

### facts

Display gathered system facts.

```bash
hostcfg facts                    # Output in HCL format (default)
hostcfg facts --format json      # Output in JSON format
hostcfg facts --format yaml      # Output in YAML format
hostcfg facts --no-env           # Exclude environment variables from output
```

### validate

Check HCL syntax and validate resource configurations.

```bash
hostcfg validate
hostcfg validate -c /path/to/config.hcl
```

## Global Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--config` | `-c` | Path to config file or directory (default: current directory) |
| `--var` | `-e` | Set a variable (can be used multiple times): `-e key=value` |
| `--var-file` | `-f` | Path to variable file (can be used multiple times) |
| `--no-color` | | Disable colored output |

## Variable Files

hostcfg supports loading variables from external files, similar to Terraform's `.tfvars` files. This allows different configurations for different hosts or environments.

### Auto-Loaded Files

The following files are automatically loaded if they exist in the configuration directory:

1. `hostcfg.vars.hcl` - Default variable file
2. `hostcfg.vars.hcl.local` - Local overrides (add to `.gitignore`)
3. `*.auto.vars.hcl` - Additional auto-loaded files (alphabetical order)

### Explicit Variable Files

Use `--var-file` (or `-f`) to load specific variable files:

```bash
hostcfg apply --var-file production.vars.hcl
hostcfg apply -f staging.vars.hcl -f secrets.vars.hcl
```

### Variable File Format

Variable files use HCL syntax with simple assignments:

```hcl
# production.vars.hcl
environment = "production"
app_port    = 8080
debug       = false

# Complex types are supported
allowed_hosts = ["web1.example.com", "web2.example.com"]

database = {
  host = "db.example.com"
  port = 5432
}
```

### Precedence

Variables are resolved in this order (highest to lowest priority):

1. CLI variables (`-e key=value`) - highest priority
2. Explicit var files (`--var-file`, in order specified)
3. Auto-loaded var files
4. Main config defaults (`variable "x" { default = ... }`)

### Example Usage

```bash
# Use auto-loaded hostcfg.vars.hcl
hostcfg apply

# Override with production values
hostcfg apply -f production.vars.hcl

# CLI overrides everything
hostcfg apply -f production.vars.hcl -e debug=true
```

## Configuration Files

hostcfg supports both single-file and multi-file configurations.

### Single File

```bash
hostcfg plan -c config.hcl
```

### Directory of Files

Load all `*.hcl` files from a directory and merge them:

```bash
hostcfg plan -c /etc/hostcfg/
```

This allows you to organize configuration by concern:

```
/etc/hostcfg/
├── variables.hcl      # Variable definitions
├── packages.hcl       # Package resources
├── services.hcl       # Service resources
├── files.hcl          # File and directory resources
└── cron.hcl           # Cron job resources
```

### Default Behavior

When no `-c` flag is specified, hostcfg loads all `*.hcl` files from the current directory.

## Example Output

```
$ hostcfg plan

~ file.app_config
    ~ mode: "0600" => "0644"
    ~ content: (changed)
        {
      -   "debug": true,
      +   "debug": false,
          "version": "1.0.0"
        }

+ file.new_config
    + path = "/etc/myapp/new.conf"
    + owner = "root"
    + mode = "0644"

- file.old_config
    - path = "/etc/myapp/old.conf"

Plan: 1 to add, 1 to change, 1 to destroy.
```
