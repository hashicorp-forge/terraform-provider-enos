output "instance_public_ips" {
  description = "Public IP of remote host"
  value       = aws_instance.remotehost.public_ip
}
