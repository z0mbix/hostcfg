# Nginx web server role with verifiable resources

# Install nginx package
resource "package" "nginx" {
  description = "Install nginx web server"
  name        = "nginx"
  ensure      = "present"
}

# Create document root
resource "directory" "document_root" {
  description = "Create document root directory"
  path        = var.document_root
  mode        = "0755"
}

# Create a simple index page
resource "file" "index_page" {
  description = "Create index.html"
  path        = "${var.document_root}/index.html"
  content     = <<-EOF
    <!DOCTYPE html>
    <html>
    <head>
        <title>Welcome to ${var.server_name}</title>
    </head>
    <body>
        <h1>Server: ${var.server_name}</h1>
        <p>This server is managed by hostcfg.</p>
    </body>
    </html>
  EOF
  mode        = "0644"

  depends_on = ["directory.document_root"]
}

# Configure nginx virtual host
resource "file" "nginx_vhost" {
  description = "Configure nginx virtual host"
  path        = "/etc/nginx/sites-available/${var.server_name}"
  content     = <<-EOF
    server {
        listen ${var.listen_port};
        server_name ${var.server_name};
        root ${var.document_root};
        index index.html index.htm;

        location / {
            try_files $uri $uri/ =404;
        }
    }
  EOF
  mode        = "0644"

  depends_on = ["package.nginx"]
}

# Enable the site (create symlink)
resource "link" "enable_site" {
  description = "Enable nginx site"
  path        = "/etc/nginx/sites-enabled/${var.server_name}"
  target      = "/etc/nginx/sites-available/${var.server_name}"

  depends_on = ["file.nginx_vhost"]
}

# Ensure nginx service is running and enabled
resource "service" "nginx" {
  description = "Ensure nginx is running and enabled"
  name        = "nginx"
  ensure      = "running"
  enabled     = true

  depends_on = ["link.enable_site"]
}
