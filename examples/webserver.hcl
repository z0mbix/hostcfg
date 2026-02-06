# Example: Configure a web server (nginx)

variable "server_name" {
  default = "example.com"
}

variable "document_root" {
  default = "/var/www/html"
}

# Install nginx package
resource "package" "nginx" {
  name   = "nginx"
  ensure = "present"
}

# Create document root directory
resource "directory" "docroot" {
  path      = var.document_root
  owner     = "www-data"
  group     = "www-data"
  mode      = "0755"
  recursive = true

  depends_on = ["package.nginx"]
}

# Create a simple index page
resource "file" "index" {
  path    = "${var.document_root}/index.html"
  content = <<-EOF
    <!DOCTYPE html>
    <html>
    <head>
      <title>Welcome to ${var.server_name}</title>
    </head>
    <body>
      <h1>Welcome to ${var.server_name}!</h1>
      <p>Managed by hostcfg</p>
    </body>
    </html>
  EOF
  owner   = "www-data"
  group   = "www-data"
  mode    = "0644"

  depends_on = ["directory.docroot"]
}

# Configure nginx virtual host
resource "file" "nginx_vhost" {
  path    = "/etc/nginx/sites-available/${var.server_name}"
  content = <<-EOF
    server {
        listen 80;
        server_name ${var.server_name};
        root ${var.document_root};
        index index.html;

        location / {
            try_files $uri $uri/ =404;
        }
    }
  EOF
  mode    = "0644"

  depends_on = ["package.nginx"]
}

# Enable the site (create symlink using exec)
resource "exec" "enable_site" {
  command = "ln -sf /etc/nginx/sites-available/${var.server_name} /etc/nginx/sites-enabled/"
  creates = "/etc/nginx/sites-enabled/${var.server_name}"

  depends_on = ["file.nginx_vhost"]
}

# Ensure nginx is running and enabled
resource "service" "nginx" {
  name    = "nginx"
  ensure  = "running"
  enabled = true

  depends_on = ["exec.enable_site"]
}
