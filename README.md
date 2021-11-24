![Validation](https://github.com/hashicorp/enos-provider/actions/workflows/validate.yml/badge.svg)

# enos-provider
A terraform provider for quality infrastructure

- [Example](#example)
- [Supported Platforms](#supported-platforms)
- [Installing the provider](#installing-the-provider)
  - [Network mirror](#network-mirror)
  - [Build from source](#build-from-source)
- [Creating new sources](#creating-new-sources)
- [Publishing to the network mirror](#publishing-to-the-network-mirror)
  - [S3 bucket access](#s3-bucket-access)
  - [Publishing the artifacts](#publishing-the-artifacts)
- [Provider Configuration](#provider-configuration)
- [Data Sources](#data-sources)
  - [enos_environment](#enos_environment)
  - [enos_artifactory_item](#enos_artifactory_item)
- [Resources](#resources)
  - [enos_file](#enos_file)
  - [enos_remote_exec](#enos_remote_exec)
  - [enos_local_exec](#enos_local_exec)
  - [enos_bundle_install](#enos_bundle_install)
  - [enos_vault_start](#enos_vault_start)
  - [enos_vault_init](#enos_vault_init)
  - [enos_vault_unseal](#enos_vault_unseal)
- [Flight control](#flight-control)
  - [Commands](#commands)
    - [Download](#download)
    - [Unzip](#unzip)
  - [Remote flight](#remote-flight)
- [Release Workflow:](#release-workflow)
    - [Validate](#validate)
    - [Release](#release)
    - [Test Current Release](#test-current-release)
    - [Promote](#promote)
  - [Artifact publishing to `enos-provider-current` S3 bucket](#artifact-publishing-to-enos-provider-current-s3-bucket)
  - [Artifact publishing to `enos-provider-stable` S3 bucket](#artifact-publishing-to-enos-provider-stable-s3-bucket)

# Example

You can find an example of how to use the enos provider in the [examples/core](https://github.com/hashicorp/enos-provider/blob/main/examples/core/) section of the repository.

# Supported Platforms

The following table show the version compatability for different platform and architectures.

| Tool/Feature            | Platforms     | Architecture | Version  | 
|-------------------------|---------------|--------------|----------|
| enos terraform provider | linux, darwin | amd64        | all      |
| enos terraform provider | linux, darwin | arm64        | <=0.1.13 |
| flight-control          | linux, darwin | amd64        | all      |
| flight-control          | linux         | arm64        | <=0.1.13 |

# Installing the provider

The provider is intended to be used as an internal testing tool, as such we want
to keep it off the public registry. Since we don't have an internal provider registry
there are two methods of installing the provider:

## Network mirror

The easiest method is to install the latest stable version from the S3 network
mirror. To do so you'll need to drop some configuration into `~/.terraformrc`

```hcl
provider_installation {
  network_mirror {
    url = "https://enos-provider.s3-us-west-2.amazonaws.com/"
  }
  direct {
    exclude = [
      "hashicorp.com/qti/enos"
    ]
  }
}
```

This configuration will tell Terraform to resolve plugins from the network mirror
and never attempt to pull it from the public registry.

If you prefer keeping this configuration out of your personal Terraform CLI
configuration you can write it to any file and use the `TF_CLI_CONFIG_FILE`
environment variable to tell Terraform where the configuration is located.

## Build from source

Another option is to build the provider from source and copy it into your provider
cache.

For local development, first you will need to build flight control. If you don't have `upx` installed, do this now: `brew install upx`. Next, run `make flight-control install` in the root of this repository.

Alternatively, you can skip installing `upx` and run `make flight-control-build install` instead. Just keep in mind that the resulting binaries will be much larger this way as they won't be compressed with `upx`.

You only need to build flight control once. Subsequently, you can just run `make` or `make install` to rebuild the provider. If you modify flight control, you will need to re-run `make flight-control`.

NOTE: If you use this method, all modules that use the enos provider will have to
explicitly specify and configure the enos provider, inheritance won't work.

# Creating new sources
To ease the burden when creating new resources and datasources, we have a scaffolding
generator that can take the name of the resource you wish to create, along with
the source type (resource or datasource), and output the scaffolding of a new
source for you. Simply run the following command and then address all the `TODO`
statements in your newly generated source.

From the root directory of this repo, run:
```shell
go run ./tools/create-source -name <your_resource_name> -type <resource|datasource>
```

Note that you should not prepend it with enos_, the utility will do that for you.

# Publishing to the network mirror

In order to provider a network mirror we have to convert our build artifacts into
archives and metadata that Terraform can remotely access. To do this you'll need
access to the S3 bucket and provider developer toolchain installed.

## S3 bucket access

The enos-provider S3 bucket resides in the `vault_team_dev` account. You will
need a `developer` role on that account to gain write access to the bucket. To
download the artifacts from the S3 bucket you'll need your IP address added to
the bucket policy that is maintained in the [mirror](./mirror) section of the
repository.

## Publishing the artifacts

To publish you will need write access to the S3 bucket, your IP address allowlisted
in the S3 bucket policy (see previous section), the `go` compiler installed, and
`docker` installed and running. All of the following commands should be run from
the root of the repository.

1. Increment the version specified in the [VERSION](./VERSION) file.
  If you fail to do this you'll overwrite the existing artifacts that exist for
  that version.
1. Remove any previous build artifacts. Run `rm ./dist/*`
1. Build the release artifacts. Run `CI=true make`
1. Publish the artifacts. Run `go run ./tools/populate-mirror -dist ./dist -bucket enos-provider`

# Provider Configuration

Currently you can provide transport level configuration at the provider level
and it will be inherited in all resource transport. If you define the same configuration
key on both levels, the resources definition will win. You may also configure
the provider level transport options in the HCL terraform file or via environment
variables. If provider configuration is defined at both levels the environment
will win.

Configuration precendence: Resource HCL > Provider Environment > Provider HCL

The following configuration parameters are supported at the provider level:

|ssh transport key|environment variable|
|-|-|
|user|ENOS_TRANSPORT_USER|
|host|ENOS_TRANSPORT_HOST|
|private_key|ENOS_TRANSPORT_PRIVATE_KEY|
|private_key_path|ENOS_TRANSPORT_PRIVATE_KEY_PATH|
|passphrase|ENOS_TRANSPORT_PASSPHRASE|
|passphrase_path|ENOS_TRANSPORT_PASSPHRASE_PATH|

While `passphrase` and `private_key` are supported, it suggested to use the
`passphrase_path` and `private_key_path` options instead, as the raw values
will be stored in Terraform state.

Example configuration
```hcl
provider "enos" {
  transport = {
    ssh = {
      user             = "ubuntu"
      private_key_path = "/path/to/your/private/key.pem"
      host             = "192.168.0.1"
    }
  }
}
```

# Data Sources

The provider provides the following datasources.

## enos_environment
The enos_environment datasource is a datasource that we can use to pass environment
specific information into our Terraform run. As enos relies on SSH to execute
the bulk of it's actions, we a common problem is granting access to the host
executing the Terraform run. As such, the enos_environment resource can be
used to pass the public_ip_address to other Terraform resources that are creating
security groups or managing firewalls.

The following describes the enos_environment schema:

|key|description|
|-|-|
|id|The id of the datasource. It is always 'static'|
|public_ip_address|The public IP address of the host executing Terraform|

Example
```hcl
data "enos_environment" "localhost" { }

module "security_group" {
  source = "terraform-aws-modules/security-group/aws//modules/ssh"

  name        = "enos_core_example"
  description = "Enos provider core example security group"
  vpc_id      = data.aws_vpc.default.id

  ingress_cidr_blocks = ["${data.enos_environment.localhost.public_ip_address}/32"]
}
```

## enos_artifactory_item
The enos_environment datasource is a datasource that we can use to to search
for items in artifactory. This is useful for finding build artifact URLs
that we can then install. The datasource will return URLs to all matching
items. The more specific your search criteria, the fewer results you'll receive.

Note: the underlying implementation uses AQL to search for artifacts and uses
the `$match` operator for every criteria. This means that you can use wildcards
`*` for any field. See the [AQL developer guide](https://www.jfrog.com/confluence/display/JFROG/Artifactory+Query+Language)
for more information.

The following describes the enos_environment schema:

|key|description|
|-|-|
|id|The id of the datasource. It is always 'static'|
|username|The Artifactory API username. This will likely be your hashicorp email address|
|token|The Artifactory API token. You can sign into Artifactory and generate one|
|host|The Artifactory API host. It should be the fully qualified base URL|
|repo|The Artifactory repository you want to search in|
|path|The path inside the Artifactory repository to search in|
|name|The artifact name|
|properties|A map of properties to match on|
|results|Items that were found that match the given search criteria|
|results.name|The item name|
|results.type|The item type|
|results.url|The fully qualified URL to the item|
|results.sha256|The SHA256 sum of the item|
|results.size|The size of the item|

Example
```hcl
data "enos_artifactory_item" "vault" {
  username = "some-user@hashicorp.com"
  token    = "1234abcd"

  host = "https://artifactory.hashicorp.engineering/artifactory"
  repo = "hashicorp-packagespec-buildcache-local*"
  path = "cache-v1/vault-enterprise/*"
  name = "*.zip"

  properties = {
    "EDITION"         = "ent"
    "GOARCH"          = "amd64"
    "GOOS"            = "linux"
    "artifactType"    = "package"
    "productRevision" = "f45845666b4e552bfc8ca775834a3ef6fc097fe0"
    "productVersion"  = "1.7.0"
  }
}

resource "enos_remote_exec" "download_vault" {
  inline  = ["curl -f --user some-user@hashicorp.com:1234abcd -o /tmp/vault.zip -X GET ${data.enos_artifactory_item.vault.results[0].url}"]

  transport = {
    ssh = {
      host             = "192.168.0.1"
      user             = "ubuntu"
      private_key_path = "/path/to/private/key.pem"
    }
  }
}
```

# Resources

The provider provides the following resources.

## enos_file
The enos file resource is capable of copying a local file to a remote destination
over an SSH transport.

The following describes the enos_file schema

|key|description|
|-|-|
|id|The id of the resource. It is always 'static'|
|sum|The SHA 256 sum of the source file. If the sum changes between runs the file will be uploaded again.|
|source|The file path to the source file to copy|
|destination|The file path on the remote host you wish to copy the file to|
|transport.ssh.host|The remote host you wish to copy the file to|
|transport.ssh.user|The username to use when performing the SSH handshake|
|transport.ssh.private_key|The text value of the private key you wish to use for SSH authentication|
|transport.ssh.private_key_path|The path of the private key you wish to use for SSH authentication|
|transport.ssh.passphrase|The text value of the passphrase for an encrypted private key|
|transport.ssh.passphrase|The path of the passphrase for an encrypted private key|

The resource is also capable of using the SSH agent. It will attempt to connect
to the agent socket as defined with the `SSH_AUTH_SOCK` environment variable.

Example
```hcl
resource "enos_file" "foo" {
  source      = "/local/path/to/file.txt"
  destination = "/remote/destination/file.txt"
  content     = data.template_file.some_template.rendered

  transport = {
    ssh = {
      host             = "192.168.0.1"
      user             = "ubuntu"
      private_key_path = "/path/to/private/key.pem"
    }
  }
}
```

## enos_remote_exec
The enos remote exec resource is capable of running scripts or commands on a
remote instance over an SSH transport.

The following describes the enos_remote_file schema:

|key|description|
|-|-|
|id|The id of the resource. It is always 'static'|
|sum|A digest of the inline commands, source files, and environment variables. If the sum changes between runs all commands will execute again|
|stdout|The aggregate STDOUT of all inline commnads, scripts, or content. Terraform's wire format cannot discern between unknown and empty values, as such, stdout will return a blank space if nothing is written to stdout|
|stderr|The aggregate STDERR of all inline commnads, scripts, or content. Terraform's wire format cannot discern between unknown and empty values, as such, stderr will return a blank space if nothing is written to stderr|
|environment|A map of key/value pairs to set as environment variable before running the commands or scripts.
|inline|An array of commands to run|
|scripts|An array of paths to scripts to run|
|transport.ssh.host|The remote host you wish to copy the file to|
|transport.ssh.user|The username to use when performing the SSH handshake|
|transport.ssh.private_key|The text value of the private key you wish to use for SSH authentication|
|transport.ssh.private_key_path|The path of the private key you wish to use for SSH authentication|
|transport.ssh.passphrase|The text value of the passphrase for an encrypted private key|
|transport.ssh.passphrase|The path of the passphrase for an encrypted private key|

The resource is also capable of using the SSH agent. It will attempt to connect
to the agent socket as defined with the `SSH_AUTH_SOCK` environment variable.

Example
```hcl
resource "enos_remote_exec" "foo" {
  environment = {
    FOO = "foo"
  }

  inline  = ["touch /tmp/inline.txt"]
  scripts = ["/local/path/to/script.sh"]
  content = data.template_file.some_template.rendered

  transport = {
    ssh = {
      host             = "192.168.0.1"
      user             = "ubuntu"
      private_key_path = "/path/to/private/key.pem"
    }
  }
}
```

## enos_local_exec
The enos local exec resource is capable of running scripts or commands locally.

The following describes the enos_local_file schema:

|key|description|
|-|-|
|id|The id of the resource. It is always 'static'|
|sum|A digest of the inline commands, source files, and environment variables. If the sum changes between runs all commands will execute again|
|stdout|The aggregate STDOUT of all inline commnads, scripts, or content. Terraform's wire format cannot discern between unknown and empty values, as such, stdout will return a blank space if nothing is written to stdout|
|stderr|The aggregate STDERR of all inline commnads, scripts, or content. Terraform's wire format cannot discern between unknown and empty values, as such, stderr will return a blank space if nothing is written to stderr|
|environment|A map of key/value pairs to set as environment variable before running the commands or scripts.
|inline|An array of commands to run|
|scripts|An array of paths to scripts to run|

Example
```hcl
resource "enos_local_exec" "foo" {
  environment = {
    FOO = "foo"
  }

  inline  = ["touch /tmp/inline.txt"]
  scripts = ["/local/path/to/script.sh"]
  content = data.template_file.some_template.rendered
}
```

## enos_bundle_install
The enos bundle install resource is capable of installing HashiCorp release bundles
from a local path, releases.hashicorp.com, or from Artifactory onto a remote node.

While all three methods of install are supported, only one can be configured at
a time.

The following describes the enos_remote_file schema:

|key|description|
|-|-|
|id|The id of the resource. It is always 'static'|
|destination|The destination directory of the installed binary, eg: /usr/local/bin/|
|path|The local path of the install bundle|
|artifactory.username|The Artifactory API username. This will likely be your hashicorp email address|
|artifactory.token|The Artifactory API token. You can sign into Artifactory and generate one|
|artifactory.url|The Artifactory item URL|
|artifactory.sha256|The Artifactory item SHA 256 sum|
|release.product|The product name that you wish to install, eg: 'vault' or 'consul'|
|release.version|The version of the product that you wish to install. Use the full semver version ('2.1.3' or 'latest'|
|release.edition|The edition of the product that you wish to install. Eg: 'oss', 'ent', 'ent.hsm', 'pro', etc.|
|transport.ssh.host|The remote host you wish to copy the file to|
|transport.ssh.user|The username to use when performing the SSH handshake|
|transport.ssh.private_key|The text value of the private key you wish to use for SSH authentication|
|transport.ssh.private_key_path|The path of the private key you wish to use for SSH authentication|
|transport.ssh.passphrase|The text value of the passphrase for an encrypted private key|
|transport.ssh.passphrase|The path of the passphrase for an encrypted private key|

The resource is also capable of using the SSH agent. It will attempt to connect
to the agent socket as defined with the `SSH_AUTH_SOCK` environment variable.

Example
```hcl
resource "enos_bundle_install" "vault" {
  # the destination is the directory when the binary will be placed
  destination = "/opt/vault/bin"

  # install from releases.hashicorp.com
  release = {
    product  = "vault"
    version  = "1.7.0"
    edition  = "ent"
  }

  # install from a local bundle
  path = "/path/to/bundle.zip"

  # install from artifactory
  artifactory = {
    username = "rcragun@hashicorp.com"
    token    = "1234abcd.."
    sha256   = "e1237bs.."
    url      = "https:/artifactory.hashicorp.engineering/artifactory/...bundle.zip"
  }

  transport = {
    ssh = {
      host             = "192.168.0.1"
      user             = "ubuntu"
      private_key_path = "/path/to/private/key.pem"
    }
  }
}
```

## enos_vault_start
The enos vault start resource is capable of configuring and starting a Vault
service. It handles creating the configuration directory, the configuration file,
the license file, the systemd unit, and starting the service.

*NOTE: Currently all `config` sub-sections must be set due to an issue with Optional attributes
in `terraform-plugin-go` and Terraform. Until it has been resolved you must set
all of them. Because of this limitation, not all configuration stanzas have been implemented yet*

The following describes the enos_vault_start schema:

|key|description|
|-|-|
|id|The id of the resource. It is always 'static'|
|bin_path|The fully qualified path to the Vault binary|
|config_dir|An optional path where Vault configuration will live. Default: '/etc/vault.d'|
|config.api_addr|The Vault [api_addr](https://www.vaultproject.io/docs/configuration#api_addr) value|
|config.cluster_addr|The Vault [cluster_addr](https://www.vaultproject.io/docs/configuration#cluster_addr) value|
|config.listener.type|The Vault [listener](https://www.vaultproject.io/docs/configuration/listener/tcp) stanza value. Currently 'tcp' is the only supported listener|
|config.listener.attributes|The Vault [listener](https://www.vaultproject.io/docs/configuration/listener/tcp#tcp-listener-parameters) parameters for the tcp listener|
|config.storage.type|The Vault [storage](https://www.vaultproject.io/docs/configuration/storage) type|
|config.storage.attributes|The Vault [storage](https://www.vaultproject.io/docs/configuration/storage) parameters for the given storage type|
|config.seal.type|The Vault [seal](https://www.vaultproject.io/docs/configuration/seal) type|
|config.seal.attributes|The Vault [seal](https://www.vaultproject.io/docs/configuration/seal) parameters for the given seal type|
|config.ui|Enable or disable the Vault UI|
|license|An optional Vault license|
|unit_name|An optional name for the systemd unit. Default: 'vault'|
|username|An optional name for the vault system user. Default: 'vault'|
|transport.ssh.host|The remote host you wish to copy the file to|
|transport.ssh.user|The username to use when performing the SSH handshake|
|transport.ssh.private_key|The text value of the private key you wish to use for SSH authentication|
|transport.ssh.private_key_path|The path of the private key you wish to use for SSH authentication|
|transport.ssh.passphrase|The text value of the passphrase for an encrypted private key|
|transport.ssh.passphrase|The path of the passphrase for an encrypted private key|

The resource is also capable of using the SSH agent. It will attempt to connect
to the agent socket as defined with the `SSH_AUTH_SOCK` environment variable.

Example
```hcl
resource "enos_vault_start" "vault" {
  bin_path       = "/opt/vault/bin/vault"

  config_dir     = "/etc/vault.d"

  config         = {
    api_addr     = "${aws_instance.target.private_ip}:8200"
    cluster_addr = "${aws_instance.target.private_ip}:8201"
    listener     = {
      type       = "tcp"
      attributes = {
        address     = "0.0.0.0:8200"
        tls_disable = "true"
      }
    }
    storage = {
      type       = "consul"
      attributes = {
        address = "127.0.0.1:8500"
        path    = "vault"
      }
    }
    seal = {
      type       = "awskms"
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
```

## enos_vault_init
The enos vault init resource is capable initializing a Vault cluster.

The following describes the enos_vault_init schema:

|key|description|
|-|-|
|id|The id of the resource. It is always 'static'|
|bin_path|The fully qualified path to the Vault binary|
|key_shares|The number of [key shares](https://www.vaultproject.io/docs/commands/operator/init#key-shares)|
|key_threshold|The [key threshold](https://www.vaultproject.io/docs/commands/operator/init#key-threshold)|
|pgp_keys|A list of [pgp keys](https://www.vaultproject.io/docs/commands/operator/init#pgp-keys)|
|root_token_pgp_key|The root token [pgp keys](https://www.vaultproject.io/docs/commands/operator/init#root-token-pgp-key)|
|recovery_shares|The number of [recovery shares](https://www.vaultproject.io/docs/commands/operator/init#recovery-shares)|
|recovery_threshold|The [recovery threshold](https://www.vaultproject.io/docs/commands/operator/init#recovery-threshold)|
|recovery_pgp_keys|A list of [recovery pgp keys](https://www.vaultproject.io/docs/commands/operator/init#recovery-pgp-keys)|
|stored_shares|The number of [stored shares](https://www.vaultproject.io/docs/commands/operator/init#stored-shares)|
|consul_auto|Enable or disable [consul auto discovery](https://www.vaultproject.io/docs/commands/operator/init#consul-auto)|
|consul_service|The name of the [consul service](https://www.vaultproject.io/docs/commands/operator/init#consul-service)|
|unseal_keys_b64|The generated unseal keys in base 64|
|unseal_keys_hex|The generated unseal keys in hex|
|unseal_keys_shares|The number of unseal key shares|
|unseal_keys_threshold|The number of unseal key shares required to unseal|
|root_token|The root token|
|recovery_keys_b64|The generated recovery keys in base 64|
|recovery_keys_hex|The generated recovery keys in hex|
|recovery_keys_shares|The number of recovery key shares|
|recovery_keys_threshold|The number of recovery key shares required to recovery|
|transport.ssh.host|The remote host you wish to copy the file to|
|transport.ssh.user|The username to use when performing the SSH handshake|
|transport.ssh.private_key|The text value of the private key you wish to use for SSH authentication|
|transport.ssh.private_key_path|The path of the private key you wish to use for SSH authentication|
|transport.ssh.passphrase|The text value of the passphrase for an encrypted private key|
|transport.ssh.passphrase|The path of the passphrase for an encrypted private key|

The resource is also capable of using the SSH agent. It will attempt to connect
to the agent socket as defined with the `SSH_AUTH_SOCK` environment variable.

Example
```hcl
resource "enos_vault_init" "vault" {
  bin_path   = "/opt/vault/bin/vault"
  vault_addr = enos_vault_start.vault.config.api_addr

  recovery_shares    = 5
  recovery_threshold = 3

  transport = {
    ssh = {
      host             = "192.168.0.1"
      user             = "ubuntu"
      private_key_path = "/path/to/private/key.pem"
    }
  }
}
```

## enos_vault_unseal
This resource will unseal a running vault cluster. For non-autounsealed vaults,
it uses the `unseal_keys_hex` from the `enos_vault_init` resource and passes them
to the appropriate `vault operator unseal` command. Auto-unsealed vaults have their
seal status checked and will restart the service if needed.

The following describes the enos_vault_unseal schema:

|key|description|
|-|-|
|id|The id of the resource. It is always 'static'|
|vault_addr|The configured `api_addr` from `vault_start`|
|seal_type|The `seal_type` from `vault_start`|
|unseal_keys|A list of `unseal_keys_hex` (or b64) provided by the output of `vault_init`|
|transport.ssh.host|The remote host you wish to copy the file to|
|transport.ssh.user|The username to use when performing the SSH handshake|
|transport.ssh.private_key|The text value of the private key you wish to use for SSH authentication|
|transport.ssh.private_key_path|The path of the private key you wish to use for SSH authentication|
|transport.ssh.passphrase|The text value of the passphrase for an encrypted private key|
|transport.ssh.passphrase|The path of the passphrase for an encrypted private key|


Example
```hcl
resource "enos_vault_unseal" "vault" {
  depends_on  = [enos_vault_init.vault]
  bin_path   = "/opt/vault/bin/vault"
  vault_addr  = enos_vault_start.vault.config.api_addr
  seal_type   = enos_vault_start.vault.config.seal.type
  unseal_keys = enos_vault_init.vault.unseal_keys_hex

  transport = {
    ssh = {
      host = aws_instance.vault_instance.public_ip
    }
  }
}
```

# Flight control
Enos works by executing remote commands on a target machine via an SSH transport
session. It's resonably safe to assume that the remote target will provide some
common POSIX commands for common tasks, however, there are some operations where
there is no common POSIX utility we can rely on, such as making remote HTTP requests
or unziping archives. While utilities that can provide those functions might
be accessible via a package manager of some sort, installing global utlities and
dealing with platform specific package managers can become a serious burden.

Rather than cargo cult a brittle and complex shell script to manage various package
managers, our solution to this problem is to bundle common operations into a binary
called `enos-flight-control`. As part of our build pipeline we build this utility
for every platform and architecture that we support and embed it into the plugin.
During runtime the provider can install it on the remote machine and then call into
it when we need advanced operations.

## Commands

### Download

The download command downloads a file from a given URL.

`enos-flight-control download --url https://some/remote/file.txt --destination /local/path/file.txt --mode 0755 --timeout 5m --sha256 02b3...`

|flags|description|
|-|-|
|url|The URL of the remote resource to download|
|destination|The destination location where the file will be written|
|mode|The file mode for the downloaded file|
|timeout|The maximum allowable time for the download operation|
|sha256|An optional SHA256 sum of the file to be downloaded. If the resulting file does not match the SHA the utility will raise an error|
|auth-user|Optional basic auth username|
|auth-password|Optional basic auth password|

### Unzip

The unzip command unzips a zip archive.

`enos-flight-control unzip --source /some/file.zip --destination /some/directory --create true`
|flags|description|
|-|-|
|source|The path to the zip archive|
|destination|The destination directory where the expanded files will be written|
|mode|The file mode for expanded files|
|create-destination|Whether or not create the destination directory if does not exist|
|destination-mode|The file mode for the destination directory if it is to be created|

## Remote flight

The `remoteflight` package provides common functions to install and operate
`enos-flight-control` on remote machines through a transport.

# Release Workflow:
This repo uses the GitHub Actions workflow for CI/CD.
This repo currently runs the following workflows:

### Validate
`Validate` workflow runs Go Lint, Build, Terraform, Unit, and Acceptance tests on each PR.

### Release
`Release` workflow is run on merge to `main` when `VERSION` is updated. This workflow publishes the Enos-provider
artifacts to `enos-provider-current` bucket in `quality_team_enos_ci` AWS account.
You can also manually trigger the Release workflow from the GitHub Actions menu in this repo.

### Test Current Release
`Test Current Release` workflow is run only on successful completion of `Release` workflow.  This workflow installs and
tests the latest Enos-provider artifact published to `enos-provider-current` bucket.

### Promote
`Promote` workflow can only be triggered manually from the GitHub Actions menu in this repo.  It requires the Provider
version to be promoted as input. This workflow installs and tests the given provider version from `enos-provider-current`
S3 bucket and publishes it to `enos-provider-stable` bucket after successful completion of the integration tests.

## Artifact publishing to `enos-provider-current` S3 bucket
The Enos-provider artifacts are built and published to `enos-provider-current` bucket in `quality_team_enos_ci` AWS account
by the `Release` workflow.

## Artifact publishing to `enos-provider-stable` S3 bucket
The Enos-provider artifacts are built and published to `enos-provider-stable` bucket in `quality_team_enos_ci` AWS account
by the `Promote` workflow.
