output "cluster_name" {
  value = local.cluster_name
}

output "hosts" {
  description = "The ec2 instance target hosts"
  value = { for idx in range(var.instance_count) : idx => {
    public_ip  = aws_instance.targets[idx].public_ip
    private_ip = aws_instance.targets[idx].private_ip
  } }
}
