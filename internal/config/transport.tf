data "enos_transport" "default" {
  ssh {
    user        = "root"
    host        = "localhost"
    private_key = "BEGIN"
  }
}
