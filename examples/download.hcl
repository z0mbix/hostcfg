# Example: Download resources
#
# Downloads files from URLs with optional checksum verification.

# Download kubectl binary with checksum verification
resource "download" "kubectl" {
  url      = "https://dl.k8s.io/release/v1.28.0/bin/linux/amd64/kubectl"
  dest     = "/usr/local/bin/kubectl"
  checksum = "sha256:4717660fd1466ec72d59000bb1d9f5cdc91fac31d491043ca62b34398e0799ce"
  mode     = "0755"
}

# Download a configuration file
resource "download" "config" {
  url     = "https://example.com/app/config.yaml"
  dest    = "/etc/myapp/config.yaml"
  owner   = "root"
  group   = "root"
  mode    = "0644"
  timeout = 60 # Custom timeout for slow connections
}

# Download with force re-download
resource "download" "always_fresh" {
  url   = "https://example.com/latest/version.txt"
  dest  = "/var/cache/version.txt"
  force = true # Always re-download
}
