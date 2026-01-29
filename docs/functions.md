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

The `template()` function renders Go templates with access to all HCL variables and resource attributes. Templates have access to all [Sprig](https://masterminds.github.io/sprig/) functions, providing Helm-style templating capabilities.

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

### Sprig Template Functions

Templates include all [Sprig functions](https://masterminds.github.io/sprig/), providing powerful templating capabilities similar to Helm.

**String Functions:**
```
{{ .var.name | upper }}                    # Convert to uppercase
{{ .var.name | lower }}                    # Convert to lowercase
{{ .var.name | title }}                    # Title case
{{ .var.text | trim }}                     # Trim whitespace
{{ .var.text | indent 4 }}                 # Indent by 4 spaces
{{ .var.text | nindent 4 }}                # Newline + indent by 4 spaces
{{ .var.list | join "," }}                 # Join list with comma
```

**Default Values:**
```
{{ .var.port | default 8080 }}             # Use 8080 if port is empty
{{ .var.config | default "{}" | toJson }}  # Default with JSON encoding
{{- if empty .var.optional }}
# optional not set
{{- end }}
```

**Encoding:**
```
{{ .var.password | b64enc }}               # Base64 encode
{{ .var.encoded | b64dec }}                # Base64 decode
{{ .var.config | toJson }}                 # Convert to JSON
{{ .var.config | toPrettyJson }}           # Pretty-print JSON
{{ .var.config | toYaml }}                 # Convert to YAML
```

**Lists and Dicts:**
```
{{ list "a" "b" "c" | join "," }}          # Create and join list
{{ .var.items | first }}                   # First element
{{ .var.items | last }}                    # Last element
{{ .var.items | uniq }}                    # Remove duplicates
{{ dict "key" "value" | toJson }}          # Create dictionary
```

**Conditionals:**
```
{{ ternary "yes" "no" .var.enabled }}      # Ternary operator
{{ .var.required | required "msg" }}       # Fail if empty
```

**Cryptographic:**
```
{{ .var.data | sha256sum }}                # SHA256 hash
{{ .var.password | htpasswd "user" }}      # Generate htpasswd entry
```

See the [Sprig documentation](https://masterminds.github.io/sprig/) for the complete list of available functions.

## Other Functions

| Function | Description | Example |
|----------|-------------|---------|
| `env(name)` | Get environment variable | `env("HOME")` |
| `coalesce(values...)` | Return first non-null value | `coalesce(var.custom, "default")` |
