---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "enos_vault_unseal Resource - terraform-provider-enos"
subcategory: ""
description: |-
  The enos_vault_unseal resource will unseal a running Vault cluster. For Vaults clusters configured
  with a shamir it uses enos_vault_init.unseal_keys_hex and passes them to the appropriate
  vault operator unseal command to unseal the cluster. For auto-unsealed Vaults clusters this
  resource simply performs a seal status check loop to ensure the cluster reaches an unsealed state
---

# enos_vault_unseal (Resource)

The `enos_vault_unseal` resource will unseal a running Vault cluster. For Vaults clusters configured
with a shamir it uses `enos_vault_init.unseal_keys_hex` and passes them to the appropriate
`vault operator unseal` command to unseal the cluster. For auto-unsealed Vaults clusters this
resource simply performs a seal status check loop to ensure the cluster reaches an unsealed state



<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `bin_path` (String) The fully qualified path to the vault binary
- `unseal_keys` (List of String, Sensitive) A list of `unseal_keys_hex` (or b64) provided by the output of `enos_vault_init`. This is only required for shamir seals
- `vault_addr` (String) The configured `api_addr` from `enos_vault_start`

### Optional

- `seal_type` (String) The `seal_type` from `enos_vault_start`. If using HA Seal provide the primary seal type
- `transport` (Dynamic) - `transport.ssh` (Object) the ssh transport configuration
- `transport.ssh.user` (String) the ssh login user|string
- `transport.ssh.host` (String) the remote host to access
- `transport.ssh.private_key` (String) the private key as a string
- `transport.ssh.private_key_path` (String) the path to a private key file
- `transport.ssh.passphrase` (String) a passphrase if the private key requires one
- `transport.ssh.passphrase_path` (String) a path to a file with the passphrase for the private key
- `transport.kubernetes` (Object) the kubernetes transport configuration
- `transport.kubernetes.kubeconfig_base64` (String) base64 encoded kubeconfig
- `transport.kubernetes.context_name` (String) the name of the kube context to access
- `transport.kubernetes.namespace` (String) the namespace of pod to access
- `transport.kubernetes.pod` (String) the name of the pod to access|string
- `transport.kubernetes.container` (String) the name of the container to access
- `transport.nomad` (Object) the nomad transport configuration
- `transport.nomad.host` (String) nomad server host, i.e. http://23.56.78.9:4646
- `transport.nomad.secret_id` (String) the nomad server secret for authenticated connections
- `transport.nomad.allocation_id` (String) the allocation id for the allocation to access
- `transport.nomad.task_name` (String) the name of the task within the allocation to access
- `unit_name` (String) The sysmted unit name if using systemd as a process manager

### Read-Only

- `id` (String) The resource identifier is always static