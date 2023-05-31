output "public_ip" {
  description = "Public IP of remote host"
  value       = aws_instance.this.public_ip
}

output "private_ip" {
  description = "Public IP of remote host"
  value       = aws_instance.this.private_ip
}
