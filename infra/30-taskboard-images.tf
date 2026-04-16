# --- ECR Repositories ---
# Shared across environments — only create in prod, look up in others.

locals {
  is_prod = var.environment == "prod"
}

resource "aws_ecr_repository" "api" {
  count                = local.is_prod ? 1 : 0
  name                 = "taskboard/api"
  image_tag_mutability = "MUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }

  tags = { Name = "taskboard-api" }
}

resource "aws_ecr_repository" "dashboard" {
  count                = local.is_prod ? 1 : 0
  name                 = "taskboard/dashboard"
  image_tag_mutability = "MUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }

  tags = { Name = "taskboard-dashboard" }
}

data "aws_ecr_repository" "api" {
  count = local.is_prod ? 0 : 1
  name  = "taskboard/api"
}

data "aws_ecr_repository" "dashboard" {
  count = local.is_prod ? 0 : 1
  name  = "taskboard/dashboard"
}

locals {
  ecr_api_url       = local.is_prod ? aws_ecr_repository.api[0].repository_url : data.aws_ecr_repository.api[0].repository_url
  ecr_api_arn       = local.is_prod ? aws_ecr_repository.api[0].arn : data.aws_ecr_repository.api[0].arn
  ecr_dashboard_url = local.is_prod ? aws_ecr_repository.dashboard[0].repository_url : data.aws_ecr_repository.dashboard[0].repository_url
  ecr_dashboard_arn = local.is_prod ? aws_ecr_repository.dashboard[0].arn : data.aws_ecr_repository.dashboard[0].arn
}

# Lifecycle policy — keep last 10 images (prod only)
resource "aws_ecr_lifecycle_policy" "api" {
  count      = local.is_prod ? 1 : 0
  repository = aws_ecr_repository.api[0].name
  policy = jsonencode({
    rules = [{
      rulePriority = 1
      description  = "Keep last 10 images"
      selection = {
        tagStatus   = "any"
        countType   = "imageCountMoreThan"
        countNumber = 10
      }
      action = { type = "expire" }
    }]
  })
}

resource "aws_ecr_lifecycle_policy" "dashboard" {
  count      = local.is_prod ? 1 : 0
  repository = aws_ecr_repository.dashboard[0].name
  policy = jsonencode({
    rules = [{
      rulePriority = 1
      description  = "Keep last 10 images"
      selection = {
        tagStatus   = "any"
        countType   = "imageCountMoreThan"
        countNumber = 10
      }
      action = { type = "expire" }
    }]
  })
}
