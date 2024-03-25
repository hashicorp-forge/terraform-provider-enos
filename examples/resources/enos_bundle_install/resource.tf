resource "enos_bundle_install" "vault" {
  # the destination is the directory when the binary will be placed. Only required for bundles
  destination = "/opt/vault/bin"

  # install from a local bundle, deb, or rpm
  path = "/path/to/bundle.zip"

  transport = {
    ssh = {
      host             = "192.168.0.1"
      user             = "ubuntu"
      private_key_path = "/path/to/private/key.pem"
    }
  }
}

resource "enos_bundle_install" "vault" {
  # the destination is the directory when the binary will be placed. Only required for bundles
  destination = "/opt/vault/bin"

  # install from releases.hashicorp.com
  release = {
    product = "vault"
    version = "1.7.0"
    edition = "ent"
  }

  # install from a local bundle
  path = "/path/to/bundle.zip"

  transport = {
    ssh = {
      host             = "192.168.0.1"
      user             = "ubuntu"
      private_key_path = "/path/to/private/key.pem"
    }
  }
}

resource "enos_bundle_install" "vault" {
  # the destination is the directory when the binary will be placed. Only required for bundles
  destination = "/opt/vault/bin"

  # install from a local bundle
  path = "/path/to/bundle.zip"

  # install from artifactory
  artifactory = {
    username = "your-user@example.org"
    token    = "1234abcd.."
    sha256   = "e1237bs.."
    # Tip: you can use the enos_artifactory_item data source to help search for this URL and then pass
    # it to the resource.
    url = "https://artifactory.example.org/artifactory/...bundle.zip"
  }

  transport = {
    ssh = {
      host             = "192.168.0.1"
      user             = "ubuntu"
      private_key_path = "/path/to/private/key.pem"
    }
  }
}
