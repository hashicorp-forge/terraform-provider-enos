resource "enos_host_info" "target" {
  transport = {
    ssh = {
      host = "192.168.0.1"
    }
  }
}

resource "enos_remote_exec" "install_my_thing" {
  environment = {
    DISTRO         = enos_host_info.target.distro
    DISTRO_VERSION = enos_host_info.target.distro_version
  }

  scripts = ["/my/installer.sh"]

  transport = {
    ssh = {
      host = "192.168.0.1"
    }
  }
}
