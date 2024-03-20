# Unseal vault with shamir keys
resource "enos_vault_unseal" "vault" {
  depends_on = [
    enos_vault_init.vault
  ]

  bin_path    = "/opt/vault/bin/vault"
  vault_addr  = enos_vault_start.vault.config.api_addr
  seal_type   = "shamir"
  unseal_keys = enos_vault_init.vault.unseal_keys_hex

  transport = {
    ssh = {
      host = aws_instance.vault_instance.public_ip
    }
  }
}

# "Unseal" vault with auto-unseal keys. Since auto-unseal should happen this resource will wait
# for it to finish.
resource "enos_vault_unseal" "vault" {
  depends_on = [
    enos_vault_init.vault
  ]

  bin_path    = "/opt/vault/bin/vault"
  vault_addr  = enos_vault_start.vault.config.api_addr
  seal_type   = "awskms"
  unseal_keys = enos_vault_init.vault.unseal_keys_hex

  transport = {
    ssh = {
      host = aws_instance.vault_instance.public_ip
    }
  }
}

# You can unseal with other transports too
resource "enos_vault_unseal" "leader" {
  depends_on = [
    enos_vault_init.leader,
  ]

  bin_path    = "/bin/vault"
  vault_addr  = local.vault_address
  seal_type   = "shamir"
  unseal_keys = enos_vault_init.leader.unseal_keys_b64

  transport = {
    kubernetes = {
      kubeconfig_base64 = var.kubeconfig_base64
      context_name      = var.context_name
      pod               = data.enos_kubernetes_pods.vault_pods.pods[0].name
      namespace         = data.enos_kubernetes_pods.vault_pods.pods[0].namespace
    }
  }
}
