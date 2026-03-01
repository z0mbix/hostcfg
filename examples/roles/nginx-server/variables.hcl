# Variables for nginx-server role

variable "server_name" {
  type        = string
  description = "Server name for nginx virtual host"
  default     = "example.com"
}

variable "listen_port" {
  type        = number
  description = "Port for nginx to listen on"
  default     = 80
}

variable "document_root" {
  type        = string
  description = "Document root directory"
  default     = "/var/www/html"
}
