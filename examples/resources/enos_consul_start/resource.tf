resource "enos_consul_start" "consul" {
  depends_on = [
    aws_instance.consul_instance,
    enos_bundle_install.consul,
  ]

  bin_path   = "/opt/consul/bin/consul"
  data_dir   = "/var/lib/consul"
  config_dir = "/etc/consul.d"
  config = {
    # Handle instance types that have multiple bind addresses
    # Template syntax : https://pkg.go.dev/github.com/hashicorp/go-sockaddr/template#pkg-overview
    # Consul bind_addr docs: https://developer.hashicorp.com/consul/docs/agent/config/config-files#bind_addr
    bind_addr        = "{{ GetPrivateInterfaces | include \"type\" \"IP\" | sort \"default\" |  limit 1 | attr \"address\"}}"
    data_dir         = "/var/lib/consul"
    datacenter       = "dc1"
    retry_join       = ["provider=aws tag_key=Type tag_value=consul-server"]
    server           = true
    bootstrap_expect = 3
    log_level        = "info"
    log_file         = "/var/log/consul.d"
  }
  # Only required for Consul Enterprise
  license   = file("/path/to/consul-enterprise.lic")
  unit_name = "consul"
  username  = "consul"

  transport = {
    ssh = {
      host = aws_instance.consul_instance[0].public_ip
    }
  }
}
