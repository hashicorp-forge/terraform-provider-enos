# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

resource "aws_instance" "vault_instance" {
  for_each               = local.vault_instances
  ami                    = var.ami_id
  instance_type          = var.instance_type
  vpc_security_group_ids = [aws_security_group.enos_vault_sg.id]
  subnet_id              = tolist(data.aws_subnets.infra.ids)[each.key % length(data.aws_subnets.infra.ids)]
  key_name               = var.ssh_aws_keypair
  iam_instance_profile   = aws_iam_instance_profile.vault_profile.name
  tags = merge(
    var.common_tags,
    {
      Name       = "${local.name_suffix}-vault-${var.vault_node_prefix}-${each.key}"
      retry_join = local.vault_cluster_tag
      Type       = local.vault_cluster_tag
    },
  )
}

resource "enos_remote_exec" "install_dependencies" {
  depends_on = [aws_instance.vault_instance]
  for_each = toset([
    for idx in local.vault_instances : idx
    if length(var.dependencies_to_install) > 0
  ])

  content = templatefile("${path.module}/templates/install-dependencies.sh", {
    dependencies = join(" ", var.dependencies_to_install)
  })

  transport = {
    ssh = {
      host = aws_instance.vault_instance[each.value].public_ip
    }
  }
}

resource "enos_bundle_install" "consul" {
  for_each = {
    for idx, instance in aws_instance.vault_instance : idx => instance
    if var.storage_backend == "consul"
  }

  destination = var.consul_install_dir
  release     = merge(var.consul_release, { product = "consul" })

  transport = {
    ssh = {
      host = each.value.public_ip
    }
  }
}

resource "enos_bundle_install" "vault" {
  for_each = aws_instance.vault_instance

  destination = var.vault_install_dir
  release     = var.vault_release == null ? var.vault_release : merge({ product = "vault" }, var.vault_release)
  artifactory = var.vault_artifactory_release
  path        = var.vault_local_artifact_path

  transport = {
    ssh = {
      host = each.value.public_ip
    }
  }
}

resource "enos_consul_start" "consul" {
  for_each = enos_bundle_install.consul

  bin_path = local.consul_bin_path
  data_dir = var.consul_data_dir
  config = {
    data_dir         = var.consul_data_dir
    datacenter       = "dc1"
    retry_join       = ["provider=aws tag_key=Type tag_value=${var.consul_cluster_tag}"]
    server           = false
    bootstrap_expect = 0
    license          = var.consul_license
    log_level        = var.consul_log_level
    log_file         = var.consul_log_dir
  }
  license   = var.consul_license
  unit_name = "consul"
  username  = "consul"

  transport = {
    ssh = {
      host = aws_instance.vault_instance[each.key].public_ip
    }
  }
}

resource "enos_vault_start" "leader" {
  depends_on = [
    enos_consul_start.consul,
    enos_bundle_install.vault,
  ]
  for_each = local.leader

  bin_path = local.vault_bin_path
  config = {
    api_addr     = "http://${aws_instance.vault_instance[each.key].private_ip}:8200"
    cluster_addr = "http://${aws_instance.vault_instance[each.key].private_ip}:8201"
    cluster_name = local.vault_cluster_tag
    listener = {
      type = "tcp"
      attributes = {
        address     = "0.0.0.0:8200"
        tls_disable = "true"
      }
      // Turn on a bunch of unathenticated options that are optional on the leader
      telemetry = {
        unauthenticated_metrics_access = true
      }
      profiling = {
        unauthenticated_pprof_access = true
      }
      inflight_requests_logging = {
        unauthenticated_in_flight_request_access = true
      }
    }
    log_level = var.vault_log_level
    storage = {
      type       = var.storage_backend
      attributes = ({ for key, value in local.storage_attributes[each.key] : key => value })
      retry_join = try(local.storage_retry_join[var.storage_backend][var.configure_retry_join], null)
    }
    // NOTE: using our seals key here and seal for followers to ensure backwards compat
    seals = {
      "primary" = local.seal[var.unseal_method]
    }
    // Turn on some telemetry options on the leader
    telemetry = {
      enable_hostname_label = true,
    }
    ui = true
  }
  config_dir     = var.vault_config_dir
  config_mode    = var.config_mode
  environment    = var.vault_environment
  license        = var.vault_license
  manage_service = var.manage_service
  username       = local.vault_service_user
  unit_name      = "vault"

  transport = {
    ssh = {
      host = aws_instance.vault_instance[each.key].public_ip
    }
  }
}

resource "enos_vault_start" "followers" {
  depends_on = [
    enos_vault_start.leader,
  ]
  for_each = local.followers

  bin_path = local.vault_bin_path
  config = {
    api_addr     = "http://${aws_instance.vault_instance[each.key].private_ip}:8200"
    cluster_addr = "http://${aws_instance.vault_instance[each.key].private_ip}:8201"
    cluster_name = local.vault_cluster_tag
    log_level    = var.vault_log_level
    listener = {
      type = "tcp"
      attributes = {
        address     = "0.0.0.0:8200"
        tls_disable = "true"
      }
    }
    storage = {
      type       = var.storage_backend
      attributes = { for key, value in local.storage_attributes[each.key] : key => value }
      retry_join = try(local.storage_retry_join[var.storage_backend][var.configure_retry_join], null)
    }
    seal = local.seal[var.unseal_method]
    ui   = true
  }
  config_dir     = var.vault_config_dir
  config_mode    = var.config_mode
  environment    = var.vault_environment
  license        = var.vault_license
  manage_service = var.manage_service
  username       = local.vault_service_user
  unit_name      = "vault"

  transport = {
    ssh = {
      host = aws_instance.vault_instance[each.key].public_ip
    }
  }
}

resource "enos_vault_init" "leader" {
  depends_on = [enos_vault_start.followers]
  for_each = toset([
    for idx in local.leader : idx
    if var.vault_init
  ])

  bin_path   = local.vault_bin_path
  vault_addr = enos_vault_start.leader[0].config.api_addr

  key_shares    = local.key_shares[var.unseal_method]
  key_threshold = local.key_threshold[var.unseal_method]

  recovery_shares    = local.recovery_shares[var.unseal_method]
  recovery_threshold = local.recovery_threshold[var.unseal_method]

  transport = {
    ssh = {
      host = aws_instance.vault_instance[0].public_ip
    }
  }
}

resource "enos_vault_unseal" "leader" {
  depends_on = [
    enos_vault_start.followers,
    enos_vault_init.leader,
  ]
  for_each    = enos_vault_init.leader // only unseal the leader if we initialized it
  bin_path    = local.vault_bin_path
  vault_addr  = enos_vault_start.leader[each.key].config.api_addr
  seal_type   = var.unseal_method
  unseal_keys = var.unseal_method != "shamir" ? null : coalesce(var.vault_unseal_keys, enos_vault_init.leader[0].unseal_keys_hex)

  transport = {
    ssh = {
      host = aws_instance.vault_instance[0].public_ip
    }
  }
}

resource "enos_vault_unseal" "followers" {
  depends_on = [
    enos_vault_init.leader,
    enos_vault_unseal.leader,
  ]
  // Only unseal followers if we're not using an auto-unseal method and we've
  // initialized the cluster
  for_each = {
    for idx, follower in local.followers : idx => follower
    if var.unseal_method == "shamir" && var.vault_init
  }
  bin_path    = local.vault_bin_path
  vault_addr  = enos_vault_start.followers[each.key].config.api_addr
  seal_type   = var.unseal_method
  unseal_keys = var.unseal_method != "shamir" ? null : coalesce(var.vault_unseal_keys, enos_vault_init.leader[0].unseal_keys_hex)

  transport = {
    ssh = {
      host = aws_instance.vault_instance[0].public_ip
    }
  }
}

// Unseal everything when we're not initializing and unseal when not init is set.
// This flag is intended to handle cases when you're adding additional nodes to
// an existing cluster to use auto-pilot to upgrade them and they're not using
// an auto-unseal method.
resource "enos_vault_unseal" "when_vault_unseal_when_no_init_is_set" {
  depends_on = [
    enos_vault_start.followers,
    enos_remote_exec.install_dependencies,
  ]
  for_each = toset([
    for idx in local.instances : idx
    if var.vault_unseal_when_no_init && !var.vault_init
  ])

  bin_path   = local.vault_bin_path
  vault_addr = "http://localhost:8200"
  seal_type  = var.unseal_method
  unseal_keys = coalesce(
    var.vault_unseal_keys,
    try(enos_vault_init.leader[0].unseal_keys_hex, null),
  )

  transport = {
    ssh = {
      host = aws_instance.vault_instance[each.key].public_ip
    }
  }
}

locals {
  private_ips = [for _, v in aws_instance.vault_instance : tostring(v.private_ip)]
}

resource "enos_remote_exec" "wait_for_leader_in_vault_hosts" {
  depends_on = [
    enos_vault_unseal.leader,
    enos_vault_unseal.followers,
    enos_remote_exec.install_dependencies,
  ]
  for_each = local.vault_instances

  environment = {
    RETRY_INTERVAL             = 2
    TIMEOUT_SECONDS            = 60
    VAULT_ADDR                 = "http://127.0.0.1:8200"
    VAULT_TOKEN                = enos_vault_init.leader[0].root_token
    VAULT_INSTANCE_PRIVATE_IPS = jsonencode(local.private_ips)
    VAULT_INSTALL_DIR          = var.vault_install_dir
  }

  scripts = [abspath("${path.module}/scripts/wait-for-leader.sh")]

  transport = {
    ssh = {
      host = aws_instance.vault_instance[each.key].public_ip
    }
  }
}

# We need to ensure that the directory used for audit logs is present and accessible to the vault
# user on all nodes, since logging will only happen on the leader.
resource "enos_remote_exec" "create_audit_log_dir" {
  depends_on = [
    enos_vault_unseal.leader,
    enos_vault_unseal.followers,
    enos_remote_exec.wait_for_leader_in_vault_hosts,
  ]
  for_each = toset([
    for idx in local.vault_instances : idx
    if var.enable_file_audit_device
  ])

  environment = {
    LOG_FILE_PATH = local.audit_device_file_path
    SERVICE_USER  = local.vault_service_user
  }

  scripts = [abspath("${path.module}/scripts/create_audit_log_dir.sh")]

  transport = {
    ssh = {
      host = aws_instance.vault_instance[each.value].public_ip
    }
  }
}

resource "enos_remote_exec" "init_audit_device" {
  depends_on = [
    enos_remote_exec.create_audit_log_dir,
    enos_vault_unseal.leader,
    enos_vault_unseal.followers,
  ]
  for_each = toset([
    for idx in local.leader : idx
    if local.enable_audit_device
  ])

  environment = {
    VAULT_TOKEN    = enos_vault_init.leader[each.key].root_token
    VAULT_ADDR     = "http://127.0.0.1:8200"
    VAULT_BIN_PATH = local.vault_bin_path
    LOG_FILE_PATH  = local.audit_device_file_path
    SERVICE_USER   = local.vault_service_user
  }

  scripts = [
    abspath("${path.module}/scripts/enable_audit_logging.sh"),
  ]

  transport = {
    ssh = {
      host = aws_instance.vault_instance[each.key].public_ip
    }
  }
}
