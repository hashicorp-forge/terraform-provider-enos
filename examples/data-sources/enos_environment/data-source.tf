data "enos_environment" "localhost" {}

module "security_group" {
  source = "terraform-aws-modules/security-group/aws/modules/ssh"

  name        = "enos_core_example"
  description = "Allow SSH to my machine"
  vpc_id      = data.aws_vpc.default.id

  ingress_cidr_blocks = [for ip in data.enos_environment.localhost.public_ipv4_addresses : "${ip}/32"]
}
