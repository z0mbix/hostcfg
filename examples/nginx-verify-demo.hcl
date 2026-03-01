# Example: Using verify to test role deployment
#
# This example demonstrates:
# 1. Deploying an nginx web server role
# 2. Verifying the configuration
# 3. Detecting and fixing drift
#
# Usage:
#   # Apply the configuration
#   hostcfg apply -c nginx-verify-demo.hcl --yes
#
#   # Verify everything is correctly configured
#   hostcfg verify -c nginx-verify-demo.hcl
#
#   # Simulate drift (manual changes)
#   sudo chmod 0777 /var/www/html/index.html
#   sudo systemctl stop nginx
#
#   # Detect drift
#   hostcfg verify -c nginx-verify-demo.hcl
#   # Should show 2 resources drifted (file mode and service state)
#
#   # Fix drift
#   hostcfg apply -c nginx-verify-demo.hcl --yes
#
#   # Verify fixed
#   hostcfg verify -c nginx-verify-demo.hcl

role "webserver" {
  source = "./roles/nginx-server"

  variables = {
    server_name   = "myapp.local"
    listen_port   = 8080
    document_root = "/var/www/myapp"
  }
}
