output "instance_ids" {
  description = "IDs of Vault instances"
  value       = [for instance in aws_instance.vault_instance : instance.id]
}

output "instance_public_ips" {
  description = "Public IPs of Vault instances"
  value       = [for instance in aws_instance.vault_instance : instance.public_ip]
}

output "instance_private_ips" {
  description = "Private IPs of Vault instances"
  value       = [for instance in aws_instance.vault_instance : instance.private_ip]
}

output "key_id" {
  value = data.aws_kms_key.kms_key.id
}

output "vault_root_token" {
  value = coalesce(var.vault_root_token, try(enos_vault_init.vault[0].root_token, null), "none")
}

output "vault_unseal_keys_b64" {
  value = try(enos_vault_init.vault[0].unseal_keys_b64, [])
}

output "vault_unseal_keys_hex" {
  value = try(enos_vault_init.vault[0].unseal_keys_hex, null)
}

output "vault_unseal_shares" {
  value = try(enos_vault_init.vault[0].unseal_keys_shares, -1)
}

output "vault_unseal_threshold" {
  value = try(enos_vault_init.vault[0].unseal_keys_threshold, -1)
}

output "vault_recovery_keys_b64" {
  value = try(enos_vault_init.vault[0].recovery_keys_b64, [])
}

output "vault_recovery_keys_hex" {
  value = try(enos_vault_init.vault[0].recovery_keys_hex, [])
}

output "vault_recovery_key_shares" {
  value = try(enos_vault_init.vault[0].recovery_keys_shares, -1)
}

output "vault_recovery_threshold" {
  value = try(enos_vault_init.vault[0].recovery_keys_threshold, -1)
}

output "vault_cluster_tag" {
  description = "Cluster tag for Vault cluster"
  value       = local.vault_cluster_tag
}