
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
      Name = "${local.name_suffix}-vault-${var.vault_node_prefix}-${each.key}"
      Type = local.vault_cluster_tag
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
      user = var.enos_transport_user
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
      user = var.enos_transport_user
      host = each.value.public_ip
    }
  }
}

resource "enos_bundle_install" "vault" {
  for_each = aws_instance.vault_instance

  destination = var.vault_install_dir
  release     = var.vault_release == null ? var.vault_release : merge(var.vault_release, { product = "vault" })
  artifactory = var.vault_artifactory_release
  path        = var.vault_local_artifact_path

  transport = {
    ssh = {
      user = var.enos_transport_user
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
    log_level        = "INFO"
    log_file         = "/var/log/consul.d"
  }
  unit_name = "consul"
  username  = "consul"

  transport = {
    ssh = {
      user = var.enos_transport_user
      host = aws_instance.vault_instance[each.key].public_ip
    }
  }
}

resource "enos_vault_start" "vault" {
  depends_on = [
    enos_consul_start.consul,
  ]
  for_each = enos_bundle_install.vault

  bin_path       = local.vault_bin_path
  config_dir     = var.vault_config_dir
  manage_service = var.manage_service
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
    }
    storage = {
      type       = var.storage_backend
      attributes = { for key, value in local.storage_config[each.key] : key => value }
    }
    seal = {
      type = "awskms"
      attributes = {
        kms_key_id = data.aws_kms_key.kms_key.id
      }
    }
    ui = true
  }
  license   = var.vault_license
  unit_name = "vault"
  username  = "vault"

  transport = {
    ssh = {
      user = var.enos_transport_user
      host = aws_instance.vault_instance[each.key].public_ip
    }
  }
}

resource "enos_vault_init" "vault" {
  depends_on = [enos_vault_start.vault]
  count      = var.vault_init ? 1 : 0

  bin_path   = local.vault_bin_path
  vault_addr = enos_vault_start.vault[0].config.api_addr

  recovery_shares    = 5
  recovery_threshold = 3

  transport = {
    ssh = {
      user = var.enos_transport_user
      host = aws_instance.vault_instance[0].public_ip
    }
  }
}

resource "enos_vault_unseal" "vault" {
  depends_on  = [enos_vault_init.vault]
  bin_path    = local.vault_bin_path
  vault_addr  = enos_vault_start.vault[0].config.api_addr
  seal_type   = enos_vault_start.vault[0].config.seal.type
  unseal_keys = null

  transport = {
    ssh = {
      user = var.enos_transport_user
      host = aws_instance.vault_instance[0].public_ip
    }
  }
}

resource "enos_remote_exec" "vault_write_license" {
  depends_on = [enos_vault_unseal.vault]

  content = templatefile("${path.module}/templates/vault-write-license.sh", {
    vault_bin_path   = local.vault_bin_path,
    vault_root_token = coalesce(var.vault_root_token, try(enos_vault_init.vault[0].root_token, null), "none")
    vault_license    = coalesce(var.vault_license, "none")
  })

  transport = {
    ssh = {
      user = var.enos_transport_user
      host = aws_instance.vault_instance[0].public_ip
    }
  }
}

resource "enos_remote_exec" "vault_verify" {
  depends_on = [enos_remote_exec.vault_write_license]
  for_each   = aws_instance.vault_instance

  content = templatefile("${path.module}/templates/vault-verify.sh", {
    vault_bin_path = local.vault_bin_path,
  })

  transport = {
    ssh = {
      user = var.enos_transport_user
      host = each.value.public_ip
    }
  }
}
