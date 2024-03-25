# Configure the provider with an SSH transport
provider "enos" {
  transport = {
    ssh = {
      user             = "ubuntu"
      private_key_path = "/path/to/my/ssh/key"
    }
  }
}

# Configure the provider with an SSH transport that requires a passphrase
provider "enos" {
  transport = {
    ssh = {
      user             = "ubuntu"
      private_key_path = "/path/to/my/ssh/key"
      passphrase_path  = "/path/to/my/passphrase"
    }
  }
}

# Configure the provider with an SSH transport with a single host
provider "enos" {
  transport = {
    ssh = {
      user             = "ubuntu"
      private_key_path = "/path/to/my/ssh/key"
      host             = "192.168.0.1"
    }
  }
}

# Configure the provider with a known Kubernetes pod
provider "enos" {
  transport = {
    kubernetes = {
      kubeconfig_base64 = "base64_encoded_kubeconfig"
      context_name      = "myk8s"
      pod               = "myapppod"
      namespace         = "mynamespace"
    }
  }
}

# Configure the provider with a known Nomad task
provider "enos" {
  transport = {
    nomad = {
      host          = "https://mycluster.example.com"
      secret_id     = "a47ed236-5a51-cadf-2ad0-4cd0fd5bc393"
      allocation_id = "68123fee-1e8b-7ecc-5b34-505ecd2dcb80"
      task_name     = "myapp"
    }
  }
}
