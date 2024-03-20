// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

const (
	datasourceStaticIDDescription = "The datasource identifier is always static"
	resourceStaticIDDescription   = "The resource identifier is always static"
)

// docCaretToBacktick takes in text and replaces ^ with `. We do this to make it possible to use
// multiline inputs with markdown plausible.
func docCaretToBacktick(in string) string {
	return strings.ReplaceAll(in, "^", "`")
}

const (
	sshTransportDescriptionKind   = tfprotov6.StringKindMarkdown
	k8sTransportDescriptionKind   = tfprotov6.StringKindMarkdown
	nomadTransportDescriptionKind = tfprotov6.StringKindMarkdown
	providerDescriptionKind       = tfprotov6.StringKindMarkdown
)

var transportsDescription = fmt.Sprintf(docCaretToBacktick(`
## SSH Transport Configuration

%s

The ^transport^ stanza for a provider or a resource has the same syntax.

## Kubernetes Transport Configuration

%s

The ^transport^ stanza for a provider or a resource has the same syntax.

## Nomad Transport Configuration

%s

The ^transport^ stanza for a provider or a resource has the same syntax.
`), sshTransportDescription, k8sTransportDescription, nomadTransportDescription)

var providerDescription = fmt.Sprintf(docCaretToBacktick(`
A terraform provider that provides resouces for powering Software Quality as Code by writing
Terraform-based quality requirement scenarios using a composable, modular, and declarative language.

It is intended to be use in conjunction with the [Enos CLI](https://github.com/hashicorp/enos) and
provide the resources necessary to use Terraform as Enos's execution engine.

The enos provider needs a configured transport to be able to perform commands on remote hosts.
The provider supports three transports: ^SSH^, ^Kubernetes^ and ^Nodad^. The ^SSH^ transport is
suitable for executing commands that would normally have been done via ^SSH^, and the ^Kubernetes^
transport can be used where the command would have been executed via ^kubectl exec^.

You can provide transport configuration at the provider level, and it will be inherited by all
resources with transport configuration. If you define the same configuration key on both levels,
the resource's definition will win. You may also configure the provider level transport options in
the HCL terraform file.

Configuration precedence: Resource HCL > Provider HCL.

A resource can only configure one transport, attempting to configure more than one will result
in a Validation error. The provider however, can configure more than one transport. This is due
to the fact that the provider level transport configuration is never used to make remote calls, but
rather is only used to provide default values for resources that require a transport. When the default
values from the provider are applied to a resource, only the default values for the transport that
the resource has configured will be applied.

%s

## Debug Diagnostics

All resources and data sources will automatically bubble up appropriate error and warning
information using Terraforms diagnostics system. Additional debugging diagnostic information such
as systemd or application logs can be useful when a resource fails. Many resources contain built-in
failure handlers which can attempt to export additional diagnostics information to aid in these
cases. This additional data will be copied from resource ^transport^ targets back to the host
machine executing enos-provider. E.g. if the ^enos_vault_start^ resource has been configured with
an ^ssh^ and ^host^ transport and it fails, if debug diagnostics are enabled we'll automatically
attempt to export systemd and journald information back to the host executing the
^terraform-provider-enos^ and Enos scenario.

To enable this behavior you'll need to configure the provider with a ^debug_data_root_dir^ attribute.
It's important to note that the information being copied could be fairly large, so you'll want to
keep an eye on the directory when using failure handler diagnostics.

The ^debug_data_root_dir^ can also be configured via the environment variable: ^ENOS_DEBUG_DATA_ROOT_DIR^.
Configuring via an environment variable will override the value configured within any ^enos^ provider
configuration block within the Terraform configuration that is being run.

For example, If I want to enable debug diagnostics and have them written to ^./enos/support/debug^,
I would configure the ^enos-provider^ with that directory as the ^debug_data_root_dir^.

^^^hcl
provider "enos" {
  debug_data_root_dir = "./enos/support/debug"
}
^^^
`), transportsDescription)

var k8sTransportDescription = docCaretToBacktick(`
The ^Kubernetes^ transport is used to execute remote commands on a container running in a ^Pod^.

The following is the supported configuration:
|key|description|type|required|
|-|-|-|-|
|transport|the transport configuration|object|yes|
|transport.kubernetes|the kubernetes transport configuration|object|yes|
|transport.kubernetes.kubeconfig_base64|base64 encoded kubeconfig|string|yes|
|transport.kubernetes.context_name|the name of the kube context to access|string|yes|
|transport.kubernetes.namespace|the namespace of pod to access|string|no, defaults to the ^default^ namespace|
|transport.kubernetes.pod|the name of the pod to access|string|yes|
|transport.kubernetes.container|the name of the container to access|string|no, defaults to the default container for the pod|

Example configuration
^^^hcl
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
^^^
`)

var sshTransportDescription = docCaretToBacktick(`
The ^SSH^ transport is used to execute remote commands on a target using the secure shell protocol.

|key|description|type|required|
|-|-|-|-|
|transport|the transport configuration|object|yes|
|transport.ssh|the ssh transport configuration|object|yes|
|transport.ssh.user|the ssh login user|string|yes|
|transport.ssh.host|the remote host to access|string|yes|
|transport.ssh.private_key|the private key as a string|string|no|
|transport.ssh.private_key_path|the path to a private key file|string|no, but if provided overrides the ^private_key^ value (if configured)|
|transport.ssh.passphrase|a passphrase if the private key requires one|string|no|
|transport.ssh.passphrase_path|a path to a file with the passphrase for the private key|string|no, but if provided overrides the ^passphrase^ value (if configured)|

While ^passphrase^ and ^private_key^ are supported, it is suggested to use the ^passphrase_path^
and ^private_key_path^ options instead, as the raw values will be stored in Terraform state.

Example configuration
^^^hcl
provider "enos" {
  transport = {
    ssh = {
      user             = "ubuntu"
      private_key_path = "/path/to/your/private/key.pem"
      host             = "192.168.0.1"
    }
  }
}
^^^
`)

var nomadTransportDescription = docCaretToBacktick(`
The ^Nomad^ transport can be used to execute remote commands on a container running in a ^Task^.

The following is the supported configuration

|key|description|type|required|
|-|-|-|-|
|transport|the transport configuration|object|yes|
|transport.nomad|the nomad transport configuration|object|yes|
|transport.nomad.host|nomad server host, i.e. http://23.56.78.9:4646|string|yes|
|transport.nomad.secret_id|the nomad server secret for authenticated connections|string|no|
|transport.nomad.allocation_id|the allocation id for the allocation to access|string|yes|
|transport.nomad.task_name|the name of the task within the allocation to access|string|yes|

Example configuration
^^^hcl
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
^^^
`)
