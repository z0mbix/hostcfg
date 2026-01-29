<!DOCTYPE html>
<html>
  <head>
    <title>{{ .var.site_name }}</title>
    <style>
      body {
        font-family: sans-serif;
        max-width: 800px;
        margin: 50px auto;
        padding: 20px;
      }
      h1 { color: #333; }
    </style>
  </head>
  <body>
    <h1>Welcome to {{ .var.site_name }}</h1>
    <p>This site is served by nginx on port {{ .var.port }}.</p>
  </body>
</html>
