variable "domain_name" {
  description = "Domain name for the Taskboard platform"
  type        = string
  default     = "taskboard.commitment-tracker-aiops-sandbox.site"
}

variable "environment" {
  description = "Environment name (dev, prod)"
  type        = string
  default     = "prod"
}

variable "db_password" {
  description = "Password for the RDS Postgres database"
  type        = string
  sensitive   = true
}

variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "eu-central-1"
}

variable "github_actions_repository" {
  description = "GitHub repository allowed to assume the deploy role, in owner/repo form. Leave empty to skip creating GitHub Actions OIDC resources."
  type        = string
  default     = ""
}

variable "github_actions_deploy_branches" {
  description = "Git branches allowed to assume the GitHub Actions deploy role."
  type        = list(string)
  default     = ["main", "develop"]
}

variable "grafana_enabled" {
  description = "Whether to provision an Amazon Managed Grafana workspace wired to CloudWatch."
  type        = bool
  default     = false
}

variable "grafana_workspace_name" {
  description = "Optional override for the Grafana workspace name."
  type        = string
  default     = ""
}

variable "grafana_authentication_providers" {
  description = "Authentication providers for the Amazon Managed Grafana workspace."
  type        = list(string)
  default     = ["AWS_SSO"]
}

variable "grafana_permission_type" {
  description = "Permission mode for the Amazon Managed Grafana workspace."
  type        = string
  default     = "SERVICE_MANAGED"
}

variable "grafana_data_sources" {
  description = "Data sources to enable in the Amazon Managed Grafana workspace."
  type        = list(string)
  default     = ["CLOUDWATCH"]
}

variable "db_instance_class" {
  description = "RDS instance class"
  type        = string
  default     = "db.t4g.small"
}

variable "api_cpu" {
  description = "CPU units for the API task (1024 = 1 vCPU)"
  type        = number
  default     = 256
}

variable "api_memory" {
  description = "Memory (MiB) for the API task"
  type        = number
  default     = 512
}

variable "dashboard_cpu" {
  description = "CPU units for the Dashboard task"
  type        = number
  default     = 256
}

variable "dashboard_memory" {
  description = "Memory (MiB) for the Dashboard task"
  type        = number
  default     = 512
}

variable "taskboard_admin_api_key" {
  description = "API key for the env-managed Taskboard admin account"
  type        = string
  sensitive   = true
  default     = ""
}

variable "taskboard_api_secrets" {
  description = "Preferred grouped Taskboard API secrets input for tfvars. Overrides taskboard_admin_api_key when set."
  type = object({
    TASKBOARD_ADMIN_API_KEY = string
  })
  sensitive = true
  default   = null
}

variable "bootstrap_admin" {
  description = "Legacy no-op input retained so older local tfvars files continue to parse cleanly."
  type        = bool
  default     = false
}
