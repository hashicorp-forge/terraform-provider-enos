terraform {
  required_providers {
    enos = {
      version = "~> 0.1"
      source   = "hashicorp.com/qti/enos"
    }
  }
}

provider "enos" { }

output "transport_user" {
  value = data.enos_transport.default.ssh.user
}

output "transport_host" {
  value = data.enos_transport.default.ssh.host
}

output "transport_private_key" {
  value = data.enos_transport.default.ssh.private_key
}

output "transport_id" {
  value = data.enos_transport.default.id
}
