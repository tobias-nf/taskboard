resource "aws_grafana_workspace" "main" {
  count = var.grafana_enabled ? 1 : 0

  name        = var.grafana_workspace_name != "" ? var.grafana_workspace_name : "${local.name_prefix}-grafana"
  description = "Taskboard observability workspace with CloudWatch access"

  account_access_type      = "CURRENT_ACCOUNT"
  authentication_providers = var.grafana_authentication_providers
  permission_type          = var.grafana_permission_type
  data_sources             = var.grafana_data_sources

  tags = {
    Name = "${local.name_prefix}-grafana"
  }
}
