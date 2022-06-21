# enos-infra
The enos_infra module is a module that creates a base infrastructure required by enos_vault_cluster and enos_consul_cluster, abstracting away the provider-specific details into outputs for use across scenarios

# Example Usage
```
module "enos_infra" {
  source             = "app.terraform.io/hashicorp-qti/aws-infra/enos"
  project_name       = var.project_name
  environment        = var.environment
  common_tags        = var.common_tags
  availability_zones = ["us-east-1a", "us-east-1f"]
}

data "aws_vpc" "infra" {
  id = var.vpc_id
}

data "aws_subnet_ids" "infra" {
  vpc_id = var.vpc_id
}

data "aws_subnet" "infra" {
  for_each = data.aws_subnet_ids.infra.ids
  id       = each.value
}


locals {
  infra_subnet_blocks = [for s in data.aws_subnet.infra : s.cidr_block]
}

resource "aws_instance" "vault" {
  # Second-newest LTS release
  ami           = module.enos_infra.ubuntu_ami_id
  instance_type = "t3.micro"

  key_name               = var.ssh_key_name
  # Lists, easy to use with `count.index`
  subnet_id              = local.infra_subnet_blocks[count.index]
```
# Inputs
* `project_name` `environment` `common_tags` - Used for resource naming/tagging
* `vpc_name` - Descriptive name for VPC resources (optional)
* `availability_zones` - List of availability zones to create VPC resouces in (defaults to all in region)
* `vpc_cidr` - IP address range to use for VPCs (optional)
* `ami_architecture` - AMI Architectures to use for searching AMI Ids (see outputs, optional)
# Outputs
## Networking
* `vpc_id` - AWS VPC ID
* `vpc_cidr` - CIDR for the entire VPC
* `vpc_subnets` - Map of IDs and CIDRs of the different subnets created in each availability zone
## OS
* `ubuntu_ami_id` - AMI ID of Ubuntu LTS (currently 18.04)
* `rhel_ami_id` - AMI ID of RHEL (currently 8.2)
## Infra
* `availability_zone_names` - List of all AZs in the region
* `account_id` - AWS Account ID
## Secrets
* `kms_key_arn` - ARN of key used to encrypt secrets
* `kms_key_alias` - Alias used for above key

# Release Workflow:
This repo uses the GitHub Actions workflow for CI/CD. Terraform fmt, plan, apply, and destroy is run on each PR.
`terraform-enos-aws-infra` is released as a private module in the Terraform Cloud `hashicorp-qti` org.
We use the https://github.com/marketplace/actions/tag-release-on-push-action?version=v0.18.0 GitHub action to
release the module based on the PR labels.

By default there is no release on merge of the PR. Following actions occur on merge(push to `main`) based on the PR label.
## For PR with label:
  * `release:patch` will create a patch release
  * `release:minor` will create a minor release
  * `release:major` will create a major release
