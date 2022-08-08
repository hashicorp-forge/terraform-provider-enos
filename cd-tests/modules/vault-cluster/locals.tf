locals {
  name_suffix       = "${var.project_name}-${var.environment}"
  vault_bin_path    = "${var.vault_install_dir}/vault"
  consul_bin_path   = "${var.consul_install_dir}/consul"
  vault_cluster_tag = coalesce(var.vault_cluster_tag, "vault-server-${random_string.cluster_id.result}")
  vault_instances   = toset([for idx in range(var.instance_count) : tostring(idx)])

  storage_config = [for idx in local.vault_instances : (var.storage_backend == "raft" ?
    merge(
      {
        node_id = "${var.vault_node_prefix}_${idx}"
      },
      var.storage_backend_addl_config
    ) :
    {
      address = "127.0.0.1:8500"
      path    = "vault"
    })
  ]
}

