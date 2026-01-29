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
| `--no-color` | | Disable colored output |

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
