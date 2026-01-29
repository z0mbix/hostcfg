# Built-in Functions

## String Functions

| Function | Description | Example |
|----------|-------------|---------|
| `upper(string)` | Convert to uppercase | `upper("hello")` → `"HELLO"` |
| `lower(string)` | Convert to lowercase | `lower("HELLO")` → `"hello"` |
| `trim(string, chars)` | Trim characters from both ends | `trim("  hello  ", " ")` → `"hello"` |
| `trimspace(string)` | Trim whitespace | `trimspace("  hello  ")` → `"hello"` |
| `replace(string, old, new)` | Replace substrings | `replace("hello", "l", "L")` → `"heLLo"` |
| `substr(string, offset, length)` | Extract substring | `substr("hello", 0, 2)` → `"he"` |
| `join(separator, list)` | Join list elements | `join(",", ["a", "b"])` → `"a,b"` |
| `split(separator, string)` | Split string into list | `split(",", "a,b")` → `["a", "b"]` |
| `format(spec, args...)` | Printf-style formatting | `format("Hello %s", "world")` |

## Collection Functions

| Function | Description |
|----------|-------------|
| `length(collection)` | Return length of list, map, or string |
| `contains(list, value)` | Check if list contains value |
| `concat(list1, list2, ...)` | Concatenate lists |
| `keys(map)` | Return map keys |
| `values(map)` | Return map values |
| `merge(map1, map2, ...)` | Merge maps |
| `sort(list)` | Sort a list of strings |
| `distinct(list)` | Remove duplicates |
| `flatten(list)` | Flatten nested lists |
| `reverse(list)` | Reverse a list |

## Numeric Functions

| Function | Description |
|----------|-------------|
| `abs(number)` | Absolute value |
| `ceil(number)` | Round up |
| `floor(number)` | Round down |
| `max(numbers...)` | Maximum value |
| `min(numbers...)` | Minimum value |

## Filesystem Functions

| Function | Description | Example |
|----------|-------------|---------|
| `file(path)` | Read file contents | `file("/etc/hostname")` |
| `template(path)` | Render Go template | `template("./config.tpl")` |
| `basename(path)` | Get filename from path | `basename("/etc/nginx/nginx.conf")` → `"nginx.conf"` |
| `dirname(path)` | Get directory from path | `dirname("/etc/nginx/nginx.conf")` → `"/etc/nginx"` |

### Template Function

The `template()` function renders Go templates with access to all HCL variables and resource attributes.

**Template file (env.tpl):**
```
APP_NAME={{ .var.app_name }}
PORT={{ .var.port }}
CONFIG_DIR={{ .directory.config_dir.path }}
```

**HCL configuration:**
```hcl
variable "app_name" {
  default = "myapp"
}

variable "port" {
  default = "8080"
}

resource "directory" "config_dir" {
  path = "/opt/myapp/config"
}

resource "file" "env_file" {
  path    = "${directory.config_dir.path}/env"
  content = template("./env.tpl")
}
```

Variables are accessible via `.var.<name>` and resources via `.<resource_type>.<resource_name>.<attribute>`.

## Other Functions

| Function | Description | Example |
|----------|-------------|---------|
| `env(name)` | Get environment variable | `env("HOME")` |
| `coalesce(values...)` | Return first non-null value | `coalesce(var.custom, "default")` |
