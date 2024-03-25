resource "enos_user" "vault" {
  name     = "vault"
  home_dir = "/etc/vault.d"
  shell    = "/bin/false"

  transport = {
    ssh = {
      host = "192.168.0.1"
    }
  }
}
