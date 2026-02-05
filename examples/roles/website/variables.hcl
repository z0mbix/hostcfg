variable "port" {
  type    = number
  default = 8000
}

variable "site_name" {
  type    = string
  default = "My Website"
}

variable "root_dir" {
  type    = string
  default = "/var/www/html"
}

variable "owner" {
  type    = string
  default = "www-data"
}

variable "group" {
  type    = string
  default = "www-data"
}

variable "ssl_enabled" {
  type    = bool
  default = false
}
