# Run inline commands on a remote target. All commands are executed in a bash shell.
resource "enos_remote_exec" "foo" {
  environment = {
    UNIT_NAME = "vault.service"
  }

  # You can use all three of these but it is advise to use one.
  inline = [
    "cloudinit status --wait",
    "systemctl status $UNIT_NAME"
  ]

  transport = {
    ssh = {
      host             = "192.168.0.1"
      user             = "ubuntu"
      private_key_path = "/path/to/private/key.pem"
    }
  }
}

# Run a local script on a remote target. All commands are executed in a bash shell.
resource "enos_remote_exec" "foo" {
  # You can use all three of these but it is advise to use one.
  scripts = ["/local/path/to/script.sh"]

  transport = {
    ssh = {
      host             = "192.168.0.1"
      user             = "ubuntu"
      private_key_path = "/path/to/private/key.pem"
    }
  }
}


# Run string content as a bash script on a remote machine.
resource "enos_remote_exec" "foo" {
  content = data.template_file.my_script.rendered

  transport = {
    ssh = {
      host             = "192.168.0.1"
      user             = "ubuntu"
      private_key_path = "/path/to/private/key.pem"
    }
  }
}

# You can use other transports as well. On k8s this uses k8s exec
resource "enos_remote_exec" "raft_join" {
  inline = [
    // asserts that vault is ready
    "for i in 1 2 3 4 5; do vault status > /dev/null 2>&1 && break || sleep 5; done",
    // joins the follower to the leader
    "vault operator raft join http://vault-0.vault-internal:8200"
  ]

  transport = {
    kubernetes = {
      kubeconfig_base64 = var.kubeconfig_base64
      context_name      = var.context_name
      pod               = data.enos_kubernetes_pods.vault_pods.pods[each.key].name
      namespace         = data.enos_kubernetes_pods.vault_pods.pods[each.key].namespace
    }
  }
}
