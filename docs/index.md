---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "enos Provider"
description: |-
  A terraform provider that provides resouces for powering Software Quality as Code by writing
  Terraform-based quality requirement scenarios using a composable, modular, and declarative language.
  It is intended to be use in conjunction with the Enos CLI https://github.com/hashicorp/enos and
  provide the resources necessary to use Terraform as Enos's execution engine.
  The enos provider needs a configured transport to be able to perform commands on remote hosts.
  The provider supports three transports: SSH, Kubernetes and Nomad. The SSH transport is
  suitable for executing commands that would normally have been done via SSH, and the Kubernetes
  transport can be used where the command would have been executed via kubectl exec.
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
  SSH Transport Configuration
  The SSH transport is used to execute remote commands on a target using the secure shell protocol.
  transport.ssh (Object) the ssh transport configurationtransport.ssh.user (String) the ssh login user|stringtransport.ssh.host (String) the remote host to accesstransport.ssh.private_key (String) the private key as a stringtransport.ssh.private_key_path (String) the path to a private key filetransport.ssh.passphrase (String) a passphrase if the private key requires onetransport.ssh.passphrase_path (String) a path to a file with the passphrase for the private key
  While passphrase and private_key are supported, it is suggested to use the passphrase_path
  and private_key_path options instead, as the raw values will be stored in Terraform state.
  Example configuration
  
  provider "enos" {
    transport = {
      ssh = {
        user             = "ubuntu"
        private_key_path = "/path/to/your/private/key.pem"
        host             = "192.168.0.1"
      }
    }
  }
  
  The transport stanza for a provider or a resource has the same syntax.
  Kubernetes Transport Configuration
  The Kubernetes transport is used to execute remote commands on a container running in a Pod.
  transport.kubernetes (Object) the kubernetes transport configurationtransport.kubernetes.kubeconfig_base64 (String) base64 encoded kubeconfigtransport.kubernetes.context_name (String) the name of the kube context to accesstransport.kubernetes.namespace (String) the namespace of pod to accesstransport.kubernetes.pod (String) the name of the pod to access|stringtransport.kubernetes.container (String) the name of the container to access
  Example configuration
  
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
  
  The transport stanza for a provider or a resource has the same syntax.
  Nomad Transport Configuration
  The Nomad transport can be used to execute remote commands on a container running in a Task.
  The following is the supported configuration
  transport.nomad (Object) the nomad transport configurationtransport.nomad.host (String) nomad server host, i.e. http://23.56.78.9:4646transport.nomad.secret_id (String) the nomad server secret for authenticated connectionstransport.nomad.allocation_id (String) the allocation id for the allocation to accesstransport.nomad.task_name (String) the name of the task within the allocation to access
  Example configuration
  
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
  
  The transport stanza for a provider or a resource has the same syntax.
  Debug Diagnostics
  All resources and data sources will automatically bubble up appropriate error and warning
  information using Terraforms diagnostics system. Additional debugging diagnostic information such
  as systemd or application logs can be useful when a resource fails. Many resources contain built-in
  failure handlers which can attempt to export additional diagnostics information to aid in these
  cases. This additional data will be copied from resource transport targets back to the host
  machine executing enos-provider. E.g. if the enos_vault_start resource has been configured with
  an ssh and host transport and it fails, if debug diagnostics are enabled we'll automatically
  attempt to export systemd and journald information back to the host executing the
  terraform-provider-enos and Enos scenario.
  To enable this behavior you'll need to configure the provider with a debug_data_root_dir attribute.
  It's important to note that the information being copied could be fairly large, so you'll want to
  keep an eye on the directory when using failure handler diagnostics.
  The debug_data_root_dir can also be configured via the environment variable: ENOS_DEBUG_DATA_ROOT_DIR.
  Configuring via an environment variable will override the value configured within any enos provider
  configuration block within the Terraform configuration that is being run.
  For example, If I want to enable debug diagnostics and have them written to ./enos/support/debug,
  I would configure the enos-provider with that directory as the debug_data_root_dir.
  
  provider "enos" {
    debug_data_root_dir = "./enos/support/debug"
  }
---

# enos Provider

A terraform provider that provides resouces for powering Software Quality as Code by writing
Terraform-based quality requirement scenarios using a composable, modular, and declarative language.

It is intended to be use in conjunction with the [Enos CLI](https://github.com/hashicorp/enos) and
provide the resources necessary to use Terraform as Enos's execution engine.

The enos provider needs a configured transport to be able to perform commands on remote hosts.
The provider supports three transports: `SSH`, `Kubernetes` and `Nomad`. The `SSH` transport is
suitable for executing commands that would normally have been done via `SSH`, and the `Kubernetes`
transport can be used where the command would have been executed via `kubectl exec`.

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


## SSH Transport Configuration


The `SSH` transport is used to execute remote commands on a target using the secure shell protocol.


- `transport.ssh` (Object) the ssh transport configuration
- `transport.ssh.user` (String) the ssh login user|string
- `transport.ssh.host` (String) the remote host to access
- `transport.ssh.private_key` (String) the private key as a string
- `transport.ssh.private_key_path` (String) the path to a private key file
- `transport.ssh.passphrase` (String) a passphrase if the private key requires one
- `transport.ssh.passphrase_path` (String) a path to a file with the passphrase for the private key

While `passphrase` and `private_key` are supported, it is suggested to use the `passphrase_path`
and `private_key_path` options instead, as the raw values will be stored in Terraform state.

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


The `Kubernetes` transport is used to execute remote commands on a container running in a `Pod`.


- `transport.kubernetes` (Object) the kubernetes transport configuration
- `transport.kubernetes.kubeconfig_base64` (String) base64 encoded kubeconfig
- `transport.kubernetes.context_name` (String) the name of the kube context to access
- `transport.kubernetes.namespace` (String) the namespace of pod to access
- `transport.kubernetes.pod` (String) the name of the pod to access|string
- `transport.kubernetes.container` (String) the name of the container to access

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


- `transport.nomad` (Object) the nomad transport configuration
- `transport.nomad.host` (String) nomad server host, i.e. http://23.56.78.9:4646
- `transport.nomad.secret_id` (String) the nomad server secret for authenticated connections
- `transport.nomad.allocation_id` (String) the allocation id for the allocation to access
- `transport.nomad.task_name` (String) the name of the task within the allocation to access

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


## Debug Diagnostics

All resources and data sources will automatically bubble up appropriate error and warning
information using Terraforms diagnostics system. Additional debugging diagnostic information such
as systemd or application logs can be useful when a resource fails. Many resources contain built-in
failure handlers which can attempt to export additional diagnostics information to aid in these
cases. This additional data will be copied from resource `transport` targets back to the host
machine executing enos-provider. E.g. if the `enos_vault_start` resource has been configured with
an `ssh` and `host` transport and it fails, if debug diagnostics are enabled we'll automatically
attempt to export systemd and journald information back to the host executing the
`terraform-provider-enos` and Enos scenario.

To enable this behavior you'll need to configure the provider with a `debug_data_root_dir` attribute.
It's important to note that the information being copied could be fairly large, so you'll want to
keep an eye on the directory when using failure handler diagnostics.

The `debug_data_root_dir` can also be configured via the environment variable: `ENOS_DEBUG_DATA_ROOT_DIR`.
Configuring via an environment variable will override the value configured within any `enos` provider
configuration block within the Terraform configuration that is being run.

For example, If I want to enable debug diagnostics and have them written to `./enos/support/debug`,
I would configure the `enos-provider` with that directory as the `debug_data_root_dir`.

```hcl
provider "enos" {
  debug_data_root_dir = "./enos/support/debug"
}
```



<!-- schema generated by tfplugindocs -->
## Schema

### Optional

- `debug_data_root_dir` (String) The root directory where failure diagnostics files (e.g. application log files) are saved.
If configured and the directory does not exist, it will be created.
If the directory is not configured, diagnostic files will not be saved locally.
- `transport` (Dynamic) - `transport.ssh` (Object) the ssh transport configuration
- `transport.ssh.user` (String) the ssh login user|string
- `transport.ssh.host` (String) the remote host to access
- `transport.ssh.private_key` (String) the private key as a string
- `transport.ssh.private_key_path` (String) the path to a private key file
- `transport.ssh.passphrase` (String) a passphrase if the private key requires one
- `transport.ssh.passphrase_path` (String) a path to a file with the passphrase for the private key
- `transport.kubernetes` (Object) the kubernetes transport configuration
- `transport.kubernetes.kubeconfig_base64` (String) base64 encoded kubeconfig
- `transport.kubernetes.context_name` (String) the name of the kube context to access
- `transport.kubernetes.namespace` (String) the namespace of pod to access
- `transport.kubernetes.pod` (String) the name of the pod to access|string
- `transport.kubernetes.container` (String) the name of the container to access
- `transport.nomad` (Object) the nomad transport configuration
- `transport.nomad.host` (String) nomad server host, i.e. http://23.56.78.9:4646
- `transport.nomad.secret_id` (String) the nomad server secret for authenticated connections
- `transport.nomad.allocation_id` (String) the allocation id for the allocation to access
- `transport.nomad.task_name` (String) the name of the task within the allocation to access
