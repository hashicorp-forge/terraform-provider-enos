# Auto-unseal
resource "enos_vault_init" "vault" {
  bin_path   = "/opt/vault/bin/vault"
  vault_addr = enos_vault_start.vault.config.api_addr

  recovery_shares    = 5
  recovery_threshold = 3

  transport = {
    ssh = {
      host             = "192.168.0.1"
      user             = "ubuntu"
      private_key_path = "/path/to/private/key.pem"
    }
  }
}

# Shamir
resource "enos_vault_init" "vault" {
  bin_path   = "/opt/vault/bin/vault"
  vault_addr = enos_vault_start.vault.config.api_addr

  key_shares    = 5
  key_threshold = 3

  transport = {
    ssh = {
      host             = "192.168.0.1"
      user             = "ubuntu"
      private_key_path = "/path/to/private/key.pem"
    }
  }
}

# You can use other transports too
resource "enos_vault_init" "leader" {
  depends_on = [
    data.enos_kubernetes_pods.vault_pods,
  ]

  bin_path   = "/bin/vault"
  vault_addr = "http://127.0.0.1:8200"

  key_shares    = 5
  key_threshold = 3

  transport = {
    kubernetes = {
      kubeconfig_base64 = var.kubeconfig_base64
      context_name      = var.context_name
      pod               = data.enos_kubernetes_pods.vault_pods.pods[0].name
      namespace         = data.enos_kubernetes_pods.vault_pods.pods[0].namespace
    }
  }
}
