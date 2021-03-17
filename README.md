![Validation](https://github.com/hashicorp/enos-provider/.github/workflows/validate.yml/badge.svg)

# enos-provider
A terraform provider for quality infrastructure

1. [Example](#example)
1. [Provider configuration](#provider-configuration)
1. [enos_environment](#enos_environment)
1. [enos_file](#enos_file)
1. [enos_remote_exec](#enos_remote_exec)
1. [Creating new sources](#creating-new-sources)

## Example

You can find an example of how to use the enos provider in the [examples/core](https://github.com/hashicorp/enos-provider/blob/main/examples/core/README.md) section of the repository. 

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
|:-|-:|
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
      user = "ubuntu"
      private_key_path = "/path/to/your/private/key.pem"
      host = "192.168.0.1"
    }
  }
}
```

# enos_environment
The enos_environment datasource is a datasource that we can use to pass environment
specific information into our Terraform run. As enos relies on SSH to execute
the bulk of it's actions, we a common problem is granting access to the host
executing the Terraform run. As such, the enos_environment resource can be
used to pass the public_ip_address to other Terraform resources that are creating
security groups or managing firewalls.

The following describes the enos_file schema:

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

# enos_file
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

  transport = {
    ssh = {
      host = "192.168.0.1"
      user = "ubuntu"
      private_key_path = "/path/to/private/key.pem"
    }
  }
}
```

# enos_remote_exec
The enos remote exec resource is capable of running scripts or commands on a
remote instance over an SSH transport.

The following describes the enos_remote_file schema

|key|description|
|-|-|
|id|The id of the resource. It is always 'static'|
|sum|A digest of the inline commands, source files, and environment variables. If the sum changes between runs all commands will execute again|
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
  inline  = ["touch /tmp/inline.txt"]
  scripts = ["/local/path/to/script.sh"]

  transport = {
    ssh = {
      host = "192.168.0.1"
      user = "ubuntu"
      private_key_path = "/path/to/private/key.pem"
    }
  }
}
```

# Creating new sources
To ease the burden when creating new resources and datasources, we have a scaffolding generator that can take the name of the resource you wish to create, along with the source type (resource or datasource), and output the scaffolding of a new source for you. Simply run the following command and then address all the `TODO` statements in your newly generated source.

From the root directory of this repo, run:
```shell
go run ./tools/create_source -name <your_resource_name> -type <resource|datasource>
```

Note that you should not prepend it with enos_, the utility will do that for you.
