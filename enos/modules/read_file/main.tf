variable "file_name" {}

output "content" {
  value = file(var.file_name)
}
