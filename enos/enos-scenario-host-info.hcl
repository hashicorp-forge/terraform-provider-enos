# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

scenario "host_info" {
  matrix {
    include {
      arch    = ["arm64", "amd64"]
      distro  = ["rhel"]
      version = ["7.9", "8.8", "9.1"]
    }

    include {
      arch    = ["arm64", "amd64"]
      distro  = ["ubuntu"]
      version = ["18.04", "20.04", "22.04"]
    }

    exclude {
      arch    = ["arm64"]
      distro  = ["rhel"]
      version = ["7.9"]
    }
  }

  terraform_cli = terraform_cli.dev // Use dev overrides
  providers = [
    provider.enos.ubuntu,
    provider.enos.rhel
  ]

  locals {
    providers = {
      rhel   = provider.enos.rhel
      ubuntu = provider.enos.ubuntu
    }
  }

  step "create_vpc" {
    module = module.create_vpc
  }

  step "ec2_info" {
    module = module.ec2_info
  }

  step "create_target" {
    module     = module.target_ec2_instances
    depends_on = [step.create_vpc]

    providers = {
      enos = local.providers[matrix.distro]
    }

    variables {
      ami_id         = step.ec2_info.ami_ids[matrix.arch][matrix.distro][matrix.version]
      instance_count = 1
      instance_types = {
        // Smallest instance types you can use with both RHEL and Ubuntu AMIs
        amd64 = "t3a.micro"
        arm64 = "t4g.micro"
      }
      vpc_id = step.create_vpc.id
    }
  }

  step "test_host_info" {
    module     = module.test_host_info
    depends_on = [step.create_target]

    providers = {
      enos = local.providers[matrix.distro]
    }

    variables {
      hosts = step.create_target.hosts

      expected_arch           = matrix.arch
      expected_distro         = matrix.distro
      expected_distro_version = matrix.version
    }
  }

  output "results" {
    value = step.test_host_info.results
  }

  output "arch" {
    value = step.test_host_info.got_arch
  }

  output "distro" {
    value = step.test_host_info.got_distro
  }

  output "distro_version" {
    value = step.test_host_info.got_distro_version
  }
}
