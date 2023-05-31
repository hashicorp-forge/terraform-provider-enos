output "instance_public_ip" {
  description = "Public IP of remote host"
  value       = aws_instance.remotehost.public_ip
}
