resource "package" "nginx" {
  name   = "nginx"
  ensure = "present"
}

resource "package" "nginx_common" {
  name   = "nginx-common"
  ensure = "present"
}

resource "directory" "root" {
  path      = var.root_dir
  owner     = var.owner
  group     = var.group
  mode      = "0755"
  recursive = true
}

resource "file" "nginx_default_link" {
  path = "/etc/nginx/sites-enabled/default"
  ensure = "absent"
}

resource "file" "nginx_default" {
  path = "/etc/nginx/sites-available/default"
  ensure = "absent"
}

resource "file" "index" {
  path       = "${var.root_dir}/index.html"
  content    = template("files/index.html.tpl")
  owner      = var.owner
  group      = var.group
  mode       = "0644"
  depends_on = ["directory.root"]
}

resource "directory" "nginx_sites_available" {
  path       = "/etc/nginx/sites-available"
  owner      = "root"
  group      = "root"
  mode       = "0755"
  recursive  = true
  depends_on = ["package.nginx"]
}

resource "directory" "nginx_sites_enabled" {
  path       = "/etc/nginx/sites-enabled"
  owner      = "root"
  group      = "root"
  mode       = "0755"
  recursive  = true
  depends_on = ["package.nginx"]
}

resource "file" "nginx_config" {
  path       = "/etc/nginx/sites-available/website.conf"
  content    = template("files/nginx.conf.tpl")
  owner      = "root"
  group      = "root"
  mode       = "0644"
  depends_on = ["directory.nginx_sites_available"]
}

resource "link" "nginx_enabled" {
  path       = "/etc/nginx/sites-enabled/website.conf"
  target     = "/etc/nginx/sites-available/website.conf"
  depends_on = ["file.nginx_config", "directory.nginx_sites_enabled"]
}

resource "service" "nginx" {
  name       = "nginx"
  ensure     = "running"
  enabled    = true
  depends_on = ["link.nginx_enabled"]
}
