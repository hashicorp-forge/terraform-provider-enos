resource "aws_security_group" "consul_sg" {
  name        = "consul-sg-${random_string.cluster_id.result}"
  description = "SSH and Consul Traffic"
  vpc_id      = var.vpc_id

  # SSH
  ingress {
    from_port = 22
    to_port   = 22
    protocol  = "tcp"
    cidr_blocks = flatten([
      formatlist("%s/32", data.enos_environment.localhost.public_ipv4_addresses),
      join(",", data.aws_vpc.infra.cidr_block_associations.*.cidr_block),
    ])
  }

  ingress {
    cidr_blocks = flatten([
      formatlist("%s/32", data.enos_environment.localhost.public_ipv4_addresses),
      join(",", data.aws_vpc.infra.cidr_block_associations.*.cidr_block),
    ])
    description      = "value"
    from_port        = 8200
    to_port          = 8600
    ipv6_cidr_blocks = []
    prefix_list_ids  = []
    protocol         = "tcp"
    self             = null
    security_groups  = []
  }

  ingress {
    cidr_blocks = flatten([
      formatlist("%s/32", data.enos_environment.localhost.public_ipv4_addresses),
      join(",", data.aws_vpc.infra.cidr_block_associations.*.cidr_block),
    ])
    description      = "value"
    from_port        = 8200
    to_port          = 8600
    ipv6_cidr_blocks = []
    prefix_list_ids  = []
    protocol         = "udp"
    self             = null
    security_groups  = []
  }

  # Internal Traffic
  ingress {
    from_port = 0
    to_port   = 0
    protocol  = "-1"
    self      = true
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(
    var.common_tags,
    {
      Name = "${local.name_suffix}-consul-sg"
    },
  )
}
