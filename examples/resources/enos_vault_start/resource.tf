# Configure and start vault with consul storage and AWSKMS auto-unseal
resource "enos_vault_start" "vault" {
  bin_path   = "/opt/vault/bin/vault"
  config_dir = "/etc/vault.d"
  config = {
    api_addr     = "${aws_instance.target.private_ip}:8200"
    cluster_addr = "${aws_instance.target.private_ip}:8201"
    listener = {
      type = "tcp"
      attributes = {
        address     = "0.0.0.0:8200"
        tls_disable = "true"
      }
    }
    storage = {
      type = "consul"
      attributes = {
        address = "127.0.0.1:8500"
        path    = "vault"
      }
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
      host             = "192.168.0.1"
      user             = "ubuntu"
      private_key_path = "/path/to/private/key.pem"
    }
  }
}

# Configure and start vault with integrated storage and shamir
resource "enos_vault_start" "vault" {
  bin_path   = "/opt/vault/bin/vault"
  config_dir = "/etc/vault.d"
  config = {
    api_addr     = "${aws_instance.target.private_ip}:8200"
    cluster_addr = "${aws_instance.target.private_ip}:8201"
    listener = {
      type = "tcp"
      attributes = {
        address     = "0.0.0.0:8200"
        tls_disable = "true"
      }
    }
    storage = {
      type = "raft"
      attributes = {
        node_id = "1"
      }
    }
    seal = {
      type       = "shamir"
      attributes = null
    }
    ui = true
  }
  license   = var.vault_license
  unit_name = "vault"
  username  = "vault"
  transport = {
    ssh = {
      host             = "192.168.0.1"
      user             = "ubuntu"
      private_key_path = "/path/to/private/key.pem"
    }
  }
}

# Configure and start vault-enterprise with integrated storage and HA Seals
resource "enos_vault_start" "vault" {
  bin_path   = "/opt/vault/bin/vault"
  config_dir = "/etc/vault.d"
  config = {
    api_addr     = "${aws_instance.target.private_ip}:8200"
    cluster_addr = "${aws_instance.target.private_ip}:8201"
    listener = {
      type = "tcp"
      attributes = {
        address     = "0.0.0.0:8200"
        tls_disable = "true"
      }
    }
    storage = {
      type = "raft"
      attributes = {
        node_id = 1
      }
    }
    seal = {
      type = "awskms"
      attributes = {
        kms_key_id = data.aws_kms_key.kms_key.id
      }
    }
    seals = {
      primary = {
        type = "pkcs11"
        attributes = {
          name           = "hsm"
          priority       = "2"
          lib            = "/usr/vault/lib/libCryptoki2_64.so"
          slot           = "2305843009213693953"
          pin            = "AAAA-BBBB-CCCC-DDDD"
          key_label      = "vault-hsm-key"
          hmac_key_label = "vault-hsm-hmac-key"
        },
      }
      secondary = {
        awskms = {
          type = "awskms"
          attributes = {
            name     = "awskms"
            priority = "2"
            type     = "awskms"
            attributes = {
              kms_key_id = data.aws_kms_key.kms_key.id
            }
          }
        }
      }
    }
    ui = true
  }
  license   = var.vault_license
  unit_name = "vault"
  username  = "vault"
  transport = {
    ssh = {
      host             = "192.168.0.1"
      user             = "ubuntu"
      private_key_path = "/path/to/private/key.pem"
    }
  }
}
