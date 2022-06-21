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
  value = enos_vault_init.vault.root_token
}

output "vault_unseal_keys_b64" {
  value = enos_vault_init.vault.unseal_keys_b64
}

output "vault_unseal_keys_hex" {
  value = enos_vault_init.vault.unseal_keys_hex
}

output "vault_unseal_shares" {
  value = enos_vault_init.vault.unseal_keys_shares
}

output "vault_unseal_threshold" {
  value = enos_vault_init.vault.unseal_keys_threshold
}

output "vault_recovery_keys_b64" {
  value = enos_vault_init.vault.recovery_keys_b64
}

output "vault_recovery_keys_hex" {
  value = enos_vault_init.vault.recovery_keys_hex
}

output "vault_recovery_key_shares" {
  value = enos_vault_init.vault.recovery_keys_shares
}

output "vault_recovery_threshold" {
  value = enos_vault_init.vault.recovery_keys_threshold
}

output "vault_cluster_tag" {
  description = "Cluster tag for Vault cluster"
  value       = local.vault_cluster_tag
}
