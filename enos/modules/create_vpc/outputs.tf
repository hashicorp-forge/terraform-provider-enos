output "vpc_id" {
  value = aws_vpc.enos_vpc.id
}

output "subnet_id" {
  value = aws_subnet.enos_subnet.id
}
