output "vpc_id" {
  value = aws_vpc.main.id
}

output "alb_dns_name" {
  value = aws_lb.main.dns_name
}

output "cloudfront_domain" {
  value = aws_cloudfront_distribution.main.domain_name
}

output "cloudfront_distribution_id" {
  value = aws_cloudfront_distribution.main.id
}

output "rds_endpoint" {
  value = aws_db_instance.main.endpoint
}

output "s3_bucket_name" {
  value = aws_s3_bucket.main.id
}

output "ecr_api_url" {
  value = aws_ecr_repository.api.repository_url
}

output "ecr_dashboard_url" {
  value = aws_ecr_repository.dashboard.repository_url
}

output "route53_nameservers" {
  description = "Set these as the nameservers for the domain at your registrar"
  value       = aws_route53_zone.main.name_servers
}

output "sns_alarms_topic_arn" {
  value = aws_sns_topic.alarms.arn
}

output "ecs_cluster_name" {
  value = aws_ecs_cluster.main.name
}

output "github_actions_role_arn" {
  description = "IAM role ARN for GitHub Actions OIDC deploys. Null when github_actions_repository is unset."
  value       = local.github_actions_enabled ? aws_iam_role.github_actions_deploy[0].arn : null
}

output "grafana_workspace_id" {
  description = "Amazon Managed Grafana workspace ID. Null when grafana_enabled is false."
  value       = var.grafana_enabled ? aws_grafana_workspace.main[0].id : null
}

output "grafana_workspace_endpoint" {
  description = "Amazon Managed Grafana workspace endpoint. Null when grafana_enabled is false."
  value       = var.grafana_enabled ? aws_grafana_workspace.main[0].endpoint : null
}
