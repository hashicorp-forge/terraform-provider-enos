# Inline commands
resource "enos_local_exec" "foo" {
  environment = {
    GOOS   = "linux"
    GOARCH = "arm64"
  }

  inline = [
    "go build ./...",
    "go test ./..."
  ]
}

# Run local scripts
resource "enos_local_exec" "foo" {
  environment = {
    GOOS   = "linux"
    GOARCH = "arm64"
  }

  scripts = ["/local/path/to/script.sh"]
}

# Create a script with string content and execute it
resource "enos_local_exec" "foo" {
  content = data.template_file.some_template.rendered
}
