server {
    listen {{ .var.port }};
    server_name localhost;

    root {{ .var.root_dir }};
    index index.html;

    location / {
        try_files $uri $uri/ =404;
    }

    access_log /var/log/nginx/website_access.log;
    error_log /var/log/nginx/website_error.log;
}
