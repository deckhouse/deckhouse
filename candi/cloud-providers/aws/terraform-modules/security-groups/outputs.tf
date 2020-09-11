output "additional_security_groups" {
  value = [aws_security_group.node.id, aws_security_group.ssh-accessible.id]
}

output "load_balancer_security_group" {
  value = aws_security_group.loadbalancer.id
}
