![Validation](https://github.com/hashicorp/enos-provider/actions/workflows/validate.yml/badge.svg)

# enos-provider
A terraform provider for quality infrastructure

- [enos-provider](#enos-provider)
- [Example](#example)
- [Supported Platforms](#supported-platforms)
- [Installing the provider](#installing-the-provider)
  - [TFC provider registry](#TFC-private-registry)
  - [Build from source](#build-from-source)
- [Creating new sources](#creating-new-sources)
- [Provider Configuration](#provider-configuration)
- [Data Sources](#data-sources)
  - [enos_environment](#enos_environment)
  - [enos_artifactory_item](#enos_artifactory_item)
  - [enos_kubernetes_pods](#enos_kubernetes_pods)
- [Resources](#resources)
  - [Core](#core)
    - [enos_file](#enos_file)
    - [enos_remote_exec](#enos_remote_exec)
    - [enos_local_exec](#enos_local_exec)
    - [enos_bundle_install](#enos_bundle_install)
  - [Boundary](#boundary)
    - [enos_boundary_init](#enos_boundary_init)
    - [enos_boundary_start](#enos_boundary_start)
  - [Consul](#consul)
    - [enos_consul_start](#enos_consul_start)
  - [Vault](#vault)
    - [enos_vault_start](#enos_vault_start)
    - [enos_vault_init](#enos_vault_init)
    - [enos_vault_unseal](#enos_vault_unseal)
  - [Kubernetes](#kubernetes)
    - [enos_local_kind_cluster](#enos_local_kind_cluster)
    - [enos_kind_load_image](#enos_kind_load_image)
- [Flight control](#flight-control)
  - [Commands](#commands)
    - [Download](#download)
    - [Unzip](#unzip)
  - [Remote flight](#remote-flight)
- [Release Workflow:](#release-workflow)
    - [Validate](#validate)
    - [Release](#release)
    - [Test TFC Upload](#test-tfc-upload)
    - [Promote Enos Provider in TFC](#promote-enos-provider-in-tfc)
    - [Promote](#promote)
  - [Artifact publishing to `enosdev` private provider registry in TFC](#artifact-publishing-to-enosdev-private-provider-registry-in-tfc)
  - [Artifact publishing to `enos` private provider registry in TFC](#artifact-publishing-to-enos-private-provider-registry-in-tfc)

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
to keep it off the public registry. We install it from private Terraform provider registry.
there are two methods of installing the provider:

## TFC private registry

The easiest method is to install the latest stable version from the TFC private registry.
To do so you'll need to drop some configuration into `~/.terraformrc`

```hcl
terraform "default" {
  required_version = ">= 1.2.0"

  required_providers {
    enos = {
      source = "app.terraform.io/hashicorp-qti/enos"
    }
  }
}
```

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

# Provider Configuration

The enos provider can execute commands on remote hosts using a transport. Currently, the 
provider supports two transports, `SSH` and `Kubernetes`. The `SSH` transport is suitable for executing
commands that would normally have been done via `SSH`, and the `Kubernetes` transport can be used
where the command would have been executed via `kubectl exec`. 

Currently, you can provide transport configuration at the provider level,
and it will be inherited by all resources with transport configuration. If you define the same configuration
key on both levels, the resource's definition will win. You may also configure
the provider level transport options in the HCL terraform file.

Configuration precedence: Resource HCL > Provider HCL.

A resource can only configure one transport, attempting to configure more than one will result 
in a Validation error. The provider however, can configure more than one transport. This is due 
to the fact that the provider level transport configuration is never used to make remote calls, but 
rather is only used to provide default values for resources that require a transport. When the default
values from the provider are applied to a resource, only the default values for the transport that
the resource has configured will be applied.

## SSH Transport Configuration

|ssh transport key|description|type|required|
|-|-|-|-|
|user|the ssh user to use|string|yes|
|host|the remote host to access|string|yes|
|private_key|the private key as a string|string|no|
|private_key_path|the path to a private key file|string|no, but if provided overrides the `private_key` value (if configured)|
|passphrase|a passphrase if the private key requires one|string|no|
|passphrase_path|a path to a file with the passphrase for the private key|string|no, but if provided overrides the `passphrase` value (if configured)|

While `passphrase` and `private_key` are supported, it is suggested to use the
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

The `transport` stanza for a provider or a resource has the same syntax.

## Kubernetes Transport Configuration

The `Kubernetes` transport can be used to execute remote commands on a container running in a `Pod`.

The following is the supported configuration

|kubernetes transport key|description|type|required|
|-|-|-|-|
|kubeconfig_base64|base64 encoded kubeconfig|string|yes|
|context_name|the name of the kube context to access|string|yes|
|namespace|the namespace of pod to access|string|no, defaults to the `default` namespace|
|pod|the name of the pod to access|string|yes|
|container|the name of the container to access|string|no, defaults to the default container for the pod|

Example configuration
```hcl
provider "enos" {
  transport = {
    kubernetes = {
      kubeconfig_base64 = "5sMp1p9VyZoS4Ljyy62OJaEq3s7HAsFLfh2Ulx2hUXHzZNxrLJWyqWYxfvwr4t9cfNw"
      context_name      = "test-cluster"
      namespace         = "vault"
      pod               = "vault-0"
      container         = "vault"
    }
  }
}
```

The `transport` stanza for a provider or a resource has the same syntax.

## Nomad Transport Configuration

The `Nomad` transport can be used to execute remote commands on a container running in a `Task`.

The following is the supported configuration

|nomad transport key|description|type|required|
|-|-|-|-|
|host|nomad server host, i.e. http://23.56.78.9:4646|string|yes|
|secret_id|the nomad server secret for authenticated connections|string|no|
|allocation_id|the allocation id for the allocation to access|string|yes|
|task_name|the name of the task within the allocation to access|string|yes|

Example configuration
```hcl
provider "enos" {
  transport = {
    nomad = {
      host          = "http://127.0.0.1:4646"
      secret_id     = "some secret"
      allocation_id = "g72bc97a"
      task_name     = "vault"
    }
  }
}
```

The `transport` stanza for a provider or a resource has the same syntax.

## Diagnostics Configuration

All `enos-provider` resources and data sources will automatically bubble up
appropriate error and warning information using Terraforms diagnostics system.
Additional debugging diagnostic information such as systemd or application
logs can be useful when a resource fails. Many resources contain built-in failure
handlers which can attempt to export additional diagnostics information to aid
in these cases. This additional data will be copied from resource `transport`
targets back to the host machine executing enos-provider. E.g. if the `enos_vault_start`
resource has been configured with an `ssh` and `host` transport and it fails, we'll
automatically attempt to export systemd and journald information back to the
host executing the `enos-provider` and Enos scenario.

To enable this behavior you'll need to configure the `enos-provider` with a
`debug_data_root_dir` attribute. It's important to note that the information
being copied could be fairly large, so you'll want to keep an eye on the directory
when using failure handler diagnostics.

For example, If I want to enable failure handler diagnostics and have them
written to `./enos/support/debug`, I would configure the `enos-provider`
with that directory as the `debug_data_root_dir`.

```hcl
provider "enos" {
  debug_data_root_dir = "./enos/support/debug"
}
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

## enos_kubernetes_pods
The `enos_kubernetes_pods` datasource can be used to query a kubernetes cluster for pods, using
label and field selectors. Details on the syntax for label and field selectors can be seen here:
https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/.
The query will return a list of `PodInfo` objects, which has the following schema:

```
PodInfo{
  Name      string
  Namespace string
}
```
**Important Note:**

As this is a datasource it will be run during plan time, unless the datasource
depends on information not available till apply. Therefore, if this datasource is used in a module
where the kubernetes cluster is being created at the same time, you must make the datasource depend
either directly or indirectly on the resources required for the cluster to be created and the app 
to be deployed.

Here's an example configuration (boilerplate excluded for `brevity`) that creates a kind clutser, 
deploys a helm chart and queries for pods:

```terraform
resource "enos_local_kind_cluster" "test" {
  name            = "test"
  kubeconfig_path = "./kubeconfig"
}

resource "helm_release" "test" {
  name  = "test"
  chart = "${path.module}/helm/test"

  namespace        = "test"
  create_namespace = true

  wait = true

  depends_on = [enos_local_kind_cluster.test]
}

data "enos_kubernetes_pods" "test" {
  kubeconfig_base64 = enos_local_kind_cluster.test.kubeconfig_base64
  context_name      = enos_local_kind_cluster.test.context_name
  namespace         = helm_release.test.namespace
  label_selectors = [
    "app.kubernetes.io/instance=ci-test",
    "app.kubernetes.io/name=ci-test"
  ]
}

resource "enos_remote_exec" "create_file" {
  inline = ["touch /tmp/some_file"]

  transport = {
    kubernetes = {
      kubeconfig_base64 = enos_local_kind_cluster.test.kubeconfig_base64
      context_name      = enos_local_kind_cluster.test.context_name
      namespace         = try(data.enos_kubernetes_pods.test.pods[0].namespace, "")
      pod               = try(data.enos_kubernetes_pods.test.pods[0].name, "")
    }
  }
}
```
In this example, the datasource only runs at apply time due to these implicit dependencies:
```terraform
  kubeconfig_base64 = enos_local_kind_cluster.test.kubeconfig_base64
  context_name      = enos_local_kind_cluster.test.context_name
```

The following is the schema for the `enos_kubernetes_pods` datasource:

|key|description|
|-|-|
|id|The id of the datasource. It will match the provided context name|
|kubeconfig_base64|\[required\] - A base64 encoded kubeconfig string|
|context_name|\[required\] - The cluster context to query. The context must be present in the provided kubeconfig|
|namespace|\[optional\] - A namespace to limit the query. If not provided all namespaces will be queried|
|label_selectors|\[optional\] - A list(string) of label selectors to use when querying the cluster for pods|
|field_selectors|\[optional\] - A list(string) of field selectors to use when querying the cluster for pods|
|pods|\[output\] - a list of kubernetes `PodInfo` object, see description above |

# Resources

The provider provides the following resources.
## Core
### enos_file
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

### enos_remote_exec
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

**Note**
* Inline commands should not include double quotes, since the command will eventually be run as: `sh -c "<your command>"`.
  If a double quote must be included in the command it should be escaped as follows: `\\\"`.

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

### enos_local_exec
The enos local exec resource is capable of running scripts or commands locally.

The following describes the enos_local_file schema:

|key|description|
|-|-|
|id|The id of the resource. It is always 'static'|
|sum|A digest of the inline commands, source files, and environment variables. If the sum changes between runs all commands will execute again|
|stdout|The aggregate STDOUT of all inline commnads, scripts, or content. Terraform's wire format cannot discern between unknown and empty values, as such, stdout will return a blank space if nothing is written to stdout|
|stderr|The aggregate STDERR of all inline commnads, scripts, or content. Terraform's wire format cannot discern between unknown and empty values, as such, stderr will return a blank space if nothing is written to stderr|
|environment|A map of key/value pairs to add to the environment variables before running the commands or scripts. All existing environment variables will be inherited automatically unless the value of the 'inherit_environment' config is false.|
|inherit_environment|Whether to inherit the all the environment variables of the current shell when running the local exec script. The default value is true. |
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

### enos_bundle_install
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

## Boundary
### enos_boundary_init
The following describes the enos_boundary_init schema:

|key|description|
|-|-|
|id|The id of the resource. It is always 'static'|
|bin_path||
|config_path||
|auth_method_id|The autogenerated Boundary auth method ID|
|auth_method_name|The autogenerated Boundary auth method name|
|auth_login_name|The autogenerated Boundary auth login name|
|auth_password|The autogenerated Boundary auth password|
|auth_scope_id|The autogenerated Boundary auth scope ID|
|auth_user_id|The autogenerated Boundary auth user ID|
|auth_user_name|The autogenerated Boundary auth user name|
|host_catalog_id|The autogenerated Boundary host catalog ID|
|host_set_id|The autogenerated Boundary host set ID|
|host_id|The autogenerated Boundary host ID|
|host_type|The autogenerated Boundary host type|
|host_scope_id|The autogenerated Boundary host scope ID|
|host_catalog_name|The autogenerated Boundary host catalog name|
|host_set_name|The autogenerated Boundary host set name|
|host_name|The autogenerated Boundary host name|
|login_role_scope_id|The autogenerated Boundary login role scope ID|
|login_role_name|The autogenerated Boundary login role name|
|org_scope_id|The autogenerated Boundary org scope ID|
|org_scope_type|The autogenerated Boundary org scope type|
|org_scope_name|The autogenerated Boundary org scope name|
|project_scope_id|The autogenerated Boundary project scope ID|
|project_scope_type|The autogenerated Boundary project scope type|
|project_scope_name|The autogenerated Boundary project scope name|
|target_id|The autogenerated Boundary target ID|
|target_default_port|The autogenerated Boundary target default port|
|target_session_max_seconds|The autogenerated Boundary target session max expiration time|
|target_session_connection_limit|The autogenerated Boundary target session connection limit|
|target_type|The autogenerated Boundary target type|
|target_scope_id|The autogenerated Boundary target scope ID|
|target_name|The autogenerated Boundary target name|

### enos_boundary_start
The following describes the enos_boundary_start schema:

|key|description|
|-|-|
|id|The id of the resource. It is always 'static'|
|bin_path|The fully qualified path to the Boundary directory|
|config_path|The fully qualified path to the Boundary config directory|
|config_name|The name of the Boundary config|
|manage_service|An optional boolean value controlling if the resource manages the systemd service. Default: 'true'|
|status|The status of Boundary systemd service|
|unit_name|An optional name for the systemd unit. Default: 'boundary'|
|username|An optional name for the Boundary system user. Default: 'boundary'|

## Consul
### enos_consul_start
The enos consul start resource is capable of configuring a Consul service on a host. It handles creating the configuration directory/files, licensing, systemd and starting the service.

The following describes the enos_consul_start schema:
|key|description|
|-|-|
|id|The id of the resource. It is always 'static'|
|bin_path|The fully qualified path to the Consul binary|
|config.datacenter|The Consul [datacenter](https://www.consul.io/docs/agent/options#_datacenter) value|
|config.data_dir|The Consul [data_dir](https://www.consul.io/docs/agent/options#_data_dir) value|
|config.retry_join|The Consul [retry_join](https://www.consul.io/docs/agent/options#_retry_join) value|
|config.bootstrap_expect|The Consul [bootstrap_expect](https://www.consul.io/docs/agent/options#_bootstrap_expect) value|
|config.server|The Consul [server](https://www.consul.io/docs/agent/options#_server) value|
|config.log_file|The Consul [log_file](https://www.consul.io/docs/agent/options#_log_file) value|
|config.log_level|The Consul [log_level](https://www.consul.io/docs/agent/options#_log_level) value|
|config_dir|An optional path where the Consul config will live. Default: `/etc/consul.d`|
|data_dir|An optional path where Consul state will be stored|
|license|An optional Consul license (required for enterprise versions)|
|unit_name|An optional name for the systemd unit. Default: `consul`|
|username|An optional name for the Consul system user. Default `consul`|
## Vault
### enos_vault_start
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
|environment|An optional map of environment variables to set when running the vault service.|
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

### enos_vault_init
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

### enos_vault_unseal
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

## Kubernetes
### enos_local_kind_cluster
An `enos_local_kind_cluster` can be used to create a kind cluster locally. See https://kind.sigs.k8s.io/

The following describes the `enos_local_kind_cluster` schema:

|key|description|
|-|-|
|id|The id of the resource. Will be equal to the name of the cluster|
|name|The name of the kind cluster to create|
|kubeconfig_path|Optional, path to use for the kubeconfig file that is either created or updated|
|kubeconfig_base64|Base64 encoded kubeconfig for connecting to the kind cluster|
|client_certificate|TLS client cert for connecting to the cluster|
|client_key|TLS client key for connecting to the cluster|
|cluster_ca_certificate|TLS client ca certificate for connecting to the cluster|
|endpoint|url for connecting to the admin endpoint of the cluster|

### enos_kind_load_image
An `enos_kind_load_image` resource can be used to load a local docker image into a kind cluster. This
resource is equivalent to issuing the command:

```shell
kind load docker-image
```
See the kind docs [here](https://kind.sigs.k8s.io/docs/user/quick-start/#loading-an-image-into-your-cluster)

The following describes the `enos_kind_load_image` schema:

|key|description|
|-|-|
|id|The id of the resource. Will be equal to the name of the \[cluster-image name\]|
|cluster_name|The name of the cluster to load the image on|
|image|The name of the image to load, i.e. \[vault\]|
|tag|The tag of the image to load, i.e. \[1.10.0\]|
|archive|An archive file to load, i.e. vault-1.10.0.tar|
|loaded_images|An object matching the LoadedImageResult struct described below. The nodes field refers to the
kubernetes node names.|

```go
// LoadedImageResult info about what cluster nodes an image was loaded on
type LoadedImageResult struct {
  // Images the images that were loaded. Each image is loaded on each node
  Images []docker.ImageInfo
  // Nodes kind cluster control plane nodes where the images were loaded
  Nodes []string
}

// ImageInfo information about a docker image
type ImageInfo struct {
  Repository string
  Tags       []TagInfo
}

// TagInfo information about an image tag
type TagInfo struct {
  Tag string
  // ID docker image ID
  ID string
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
`Release` workflow is run on merge to `main` when `VERSION` is updated. This workflow publishes the Enos-provider artifacts to:
  - `enosdev` private provider registry in `hashicorp-qti` org in TFC

You can also manually trigger the Release workflow from the GitHub Actions menu in this repo.

### Test TFC Upload
`Test TFC Upload` workflow is run only on successful completion of `Release` workflow.  This workflow calls the reusable workflow `Test TFC Artifact` which installs and tests the latest Enos provider artifact installed from `enosdev` private provider registry of `hashicorp-qti` org in TFC.

### Promote Enos Provider in TFC
`Promote Enos Provider in TFC` workflow can only be triggered manually from the GitHub Actions menu in this repo.  It requires the Provider version to be promoted as input. This workflow calls the reusable workflow `Test TFC Artifact` which installs and tests the given provider version from `enosdev` private provider registry of `hashicorp-qti` org and publishes it to `enos` private provider registry of `hashicorp-qti` org in Terraform Cloud. This workflow also tests the promoted artifact using the reusable workflow `Test TFC Artifact` by installing the promoted provider version from `enos` private provider registry of `hashicorp-qti` org in TFC.

## Artifact publishing to `enosdev` private provider registry in TFC
The Enos-provider artifacts are built and published to `enosdev` private provider registry of `hashicorp-qti` org in TFC by the `Release` workflow.  This workflow uses the `tfc upload` [command](./tools/publish/README.md#tfc-upload-command)

## Artifact publishing to `enos` private provider registry in TFC
The Enos-provider artifacts are published to `enos` private provider registry of `hashicorp-qti` org in TFC by the `Promote Enos Provider in TFC` workflow.  This workflow uses the `tfc promote` [command](./tools/publish/README.md#tfc-promote-command) which downloads, renames, and publishes the tested `enosdev` registry artifacts to `enos` private provider registry.
