terraform {
  required_providers {
    enos = {
      version = "~> 0.1"
      source   = "hashicorp.com/qti/enos"
    }
  }
}

provider "enos" { }

data "enos_transport" "default" {
  ssh {
    user = "root"
    host = "localhost"
    private_key = "BEGIN"
  }
}

resource "enos_file" "long" {
  source = "/src/foo"
  destination = "/dst/foo"

  # Long form transport settings
  transport = {
    ssh = {
      user = "ubuntu"
      host = "remote"
      private_key = "BEGIN"
    }
  }
}

resource "enos_file" "short" {
  source = "/src/foo"
  destination = "/dst/bar"

  # Short form transport settings
  transport = data.enos_transport.default.out
}

output "long_transport" {
  value = enos_file.long.transport
}

output "short_transport" {
  value = enos_file.short.transport
}
