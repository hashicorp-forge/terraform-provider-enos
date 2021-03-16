# enos-provider
A terraform provider for quality infrastructure

1. [Example](#example)
1. [Provider configuration](#provider-configuration)
1. [enos_file](#enos_file)
1. [enos_remote_exec](#enos_remote_exec)
1. [Creating new resources](#creating-new-resources)

## Example
Example of using the resource to copy files and run commands against a new
Ec2 instance. It assumes that the security group name given provides SSH
access on port 22 to the host machines public IP address.

```hcl
terraform {
  required_providers {
    enos = {
      version = "~> 0.1"
      source  = "hashicorp.com/qti/enos"
    }

    aws = {
      source = "hashicorp/aws"
    }
  }
}

variable "key_name" {
  type = string
}

variable "security_group" {
  type = string
}

provider "enos" {
  transport = {
    ssh = {
      user = "ubuntu"
      private_key_path = "/path/to/your/private/key.pem"
      # or ENOS_TRANSPORT_PRIVATE_KEY_PATH=/path/to/your/private/key.pem
    }
  }
}

provider "aws" {
  region = "us-west-2"
}

data "aws_ami" "ubuntu" {
  most_recent = true

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-focal-20.04-amd64-server-*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  owners = ["099720109477"] # Canonical
}

resource "aws_instance" "target" {
  ami                         = data.aws_ami.ubuntu.id
  instance_type               = "t3.micro"
  key_name                    = var.key_name
  associate_public_ip_address = true
  security_groups             = [var.security_group]
}

resource "enos_file" "foo" {
  depends_on = [aws_instance.target]

  source      = "/path/to/file.txt"
  destination = "/tmp/bar.txt"
  transport = {
    ssh = {
      host = aws_instance.target.public_ip
    }
  }
}

resource "enos_remote_exec" "foo" {
  depends_on = [
    aws_instance.target,
    enos_file.foo
  ]

  inline  = ["cp /tmp/bar.txt /tmp/baz.txt"]
  scripts = ["${path.module}/files/script.sh"]

  transport = {
    ssh = {
      host = aws_instance.target.public_ip
    }
  }
}

terraform {
  required_providers {
    enos = {
      version = "~> 0.1"
      source   = "hashicorp.com/qti/enos"
    }
  }
}
```

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

# enos_file

The enos file resource is capable of copying a local file to a remote destination
over an SSH transport.

The following describes the enos_file schema

|key|description|
|:-|-:|
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
|:-|-:|
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

# Creating new resources
To ease the burden when creating new resources, we have a scaffolding generator that can take the name of the resource you wish to create and generates the many lines boilerplate for you. Simply run the following command and then address all the `TODO` statements in your newly generated resource.

From the root directory of this repo, run:
```shell
go run ./tools/create_resource -name <your_resource_name>
```

Note that you should not prepend it with enos_, the utility will do that for you.
