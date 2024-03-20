resource "enos_boundary_init" "controller" {
  bin_name    = "boundary"
  bin_path    = "/opt/boundary/bin"
  config_path = "/etc/boundary"
  license     = file("/path/to/boundary.lic")

  transport = {
    ssh = {
      host = aws_instance.controller[0].public_ip
    }
  }
}
