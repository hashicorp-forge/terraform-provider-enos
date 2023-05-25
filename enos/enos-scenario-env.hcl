scenario "env" {
  step "environment" {
    module = module.environment
  }

  output "public_ip" {
    value = step.environment.public_ip_address
  }

  output "public_ips" {
    value = step.environment.public_ip_addresses
  }
}
