# --- Secrets Manager ---

locals {
  taskboard_api_secret_values = var.taskboard_api_secrets != null ? var.taskboard_api_secrets : {
    TASKBOARD_ADMIN_API_KEY = var.taskboard_admin_api_key
  }
}

resource "aws_secretsmanager_secret" "taskboard_api" {
  name = "taskboard-${var.environment}-api/config"
  tags = { Name = "${local.name_prefix}-taskboard-api-secrets" }
}

resource "aws_secretsmanager_secret_version" "taskboard_api" {
  secret_id     = aws_secretsmanager_secret.taskboard_api.id
  secret_string = jsonencode(local.taskboard_api_secret_values)
}

# Grant ECS execution role access to pull secrets at container start
resource "aws_iam_role_policy" "ecs_execution_secrets" {
  name = "secrets-access"
  role = aws_iam_role.ecs_execution.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Action = ["secretsmanager:GetSecretValue"]
      Resource = [
        aws_secretsmanager_secret.taskboard_api.arn,
      ]
    }]
  })
}
