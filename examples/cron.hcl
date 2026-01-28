# Example: Configure cron jobs

variable "backup_dir" {
  default = "/var/backups"
}

variable "log_dir" {
  default = "/var/log/myapp"
}

# Create backup directory
resource "directory" "backup_dir" {
  path = var.backup_dir
  mode = "0750"
}

# Create log directory
resource "directory" "log_dir" {
  path = var.log_dir
  mode = "0755"
}

# Daily backup script
resource "file" "backup_script" {
  path    = "/usr/local/bin/daily-backup.sh"
  content = <<-EOF
    #!/bin/bash
    # Daily backup script managed by hostcfg

    DATE=$(date +%Y%m%d)
    tar -czf ${var.backup_dir}/backup-$DATE.tar.gz /etc /home

    # Keep only last 7 days
    find ${var.backup_dir} -name "backup-*.tar.gz" -mtime +7 -delete
  EOF
  mode = "0755"

  depends_on = ["directory.backup_dir"]
}

# Cron job for daily backup at 2am
resource "cron" "daily_backup" {
  command  = "/usr/local/bin/daily-backup.sh >> ${var.log_dir}/backup.log 2>&1"
  schedule = "0 2 * * *"
  user     = "root"

  depends_on = ["file.backup_script", "directory.log_dir"]
}

# Log rotation script
resource "file" "logrotate_script" {
  path    = "/usr/local/bin/rotate-logs.sh"
  content = <<-EOF
    #!/bin/bash
    # Log rotation script managed by hostcfg

    for log in ${var.log_dir}/*.log; do
      if [ -f "$log" ]; then
        gzip -c "$log" > "$log.$(date +%Y%m%d).gz"
        : > "$log"
      fi
    done

    # Keep only last 30 days of compressed logs
    find ${var.log_dir} -name "*.log.*.gz" -mtime +30 -delete
  EOF
  mode = "0755"

  depends_on = ["directory.log_dir"]
}

# Weekly log rotation at Sunday 3am
resource "cron" "weekly_logrotate" {
  command  = "/usr/local/bin/rotate-logs.sh"
  schedule = "0 3 * * 0"
  user     = "root"

  depends_on = ["file.logrotate_script"]
}
