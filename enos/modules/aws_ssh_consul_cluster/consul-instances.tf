# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

resource "aws_instance" "consul_instance" {
  for_each               = toset([for idx in range(var.instance_count) : tostring(idx)])
  ami                    = var.ami_id
  instance_type          = var.instance_type
  vpc_security_group_ids = [aws_security_group.consul_sg.id]
  subnet_id              = tolist(data.aws_subnets.infra.ids)[each.key % length(data.aws_subnets.infra.ids)]
  key_name               = var.ssh_aws_keypair
  iam_instance_profile   = aws_iam_instance_profile.consul_profile.name

  tags = merge(
    var.common_tags,
    {
      Name = "${local.name_suffix}-consul-${each.key}",
      Type = local.consul_cluster_tag,
    },
  )
}

resource "enos_bundle_install" "consul" {
  depends_on = [aws_instance.consul_instance]
  for_each   = toset([for idx in range(var.instance_count) : tostring(idx)])

  destination = var.consul_install_dir
  release     = merge(var.consul_release, { product = "consul" })

  transport = {
    ssh = {
      host = aws_instance.consul_instance[tonumber(each.value)].public_ip
    }
  }
}

resource "enos_consul_start" "consul" {
  depends_on = [
    aws_instance.consul_instance,
    enos_bundle_install.consul,
  ]

  for_each = toset([for idx in range(var.instance_count) : tostring(idx)])

  bin_path   = local.consul_bin_path
  data_dir   = var.consul_data_dir
  config_dir = var.consul_config_dir
  config = {
    # When using Amazon Linux 2 instances, multiple IPs are made available, so we
    # need to tell Consul which one to bind to. Other other supported distros do
    # not have this requirement, but this filter still returns the correct IP on
    # those, so this config can be used as is for all distros. We use the go-sockaddr
    # template to handle this: https://pkg.go.dev/github.com/hashicorp/go-sockaddr/template#pkg-overview
    # Consul bind_addr docs: https://developer.hashicorp.com/consul/docs/agent/config/config-files#bind_addr
    bind_addr        = "{{ GetPrivateInterfaces | include \"type\" \"IP\" | sort \"default\" |  limit 1 | attr \"address\"}}"
    data_dir         = var.consul_data_dir
    datacenter       = "dc1"
    retry_join       = ["provider=aws tag_key=Type tag_value=${local.consul_cluster_tag}"]
    server           = true
    bootstrap_expect = var.instance_count
    log_level        = var.consul_log_level
    log_file         = var.consul_log_dir
  }
  license   = var.consul_license
  unit_name = "consul"
  username  = "consul"

  transport = {
    ssh = {
      host = aws_instance.consul_instance[tonumber(each.value)].public_ip
    }
  }
}
