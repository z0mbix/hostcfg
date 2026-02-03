# Example: Stat resources
#
# Gathers information about files, directories, and symlinks.
# Stat is a read-only resource - it never makes changes.

# Check if a backup file exists
resource "stat" "backup" {
  path = "/etc/myapp/config.bak"
}

# Check a directory
resource "stat" "data_dir" {
  path = "/var/lib/myapp/data"
}

# Check a symlink without following it
resource "stat" "current_link" {
  path   = "/opt/myapp/current"
  follow = false
}

# Use stat results in another resource
resource "file" "status_report" {
  path    = "/tmp/status.txt"
  content = <<-EOF
    Backup file exists: ${stat.backup.exists}
    Data directory exists: ${stat.data_dir.exists}
    Current symlink exists: ${stat.current_link.exists}
  EOF
}
