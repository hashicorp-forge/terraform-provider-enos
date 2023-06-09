output "instance_ids" {
  description = "IDs of Consul instances"
  value       = [for instance in aws_instance.consul_instance : instance.id]
}

output "instance_private_ips" {
  description = "Private IPs of Consul instances"
  value       = [for instance in aws_instance.consul_instance : instance.private_ip]
}

output "instance_public_ips" {
  description = "Public IPs of Consul instances"
  value       = [for instance in aws_instance.consul_instance : instance.public_ip]
}

output "consul_cluster_tag" {
  description = "Cluster tag for Consul cluster"
  value       = local.consul_cluster_tag
}
