locals {
  github_actions_enabled = trimspace(var.github_actions_repository) != ""
}

# OIDC provider — one per account, managed by prod only
resource "aws_iam_openid_connect_provider" "github_actions" {
  count = local.github_actions_enabled && local.is_prod ? 1 : 0

  url = "https://token.actions.githubusercontent.com"

  client_id_list = ["sts.amazonaws.com"]

  tags = {
    Name = "${local.name_prefix}-github-actions-oidc"
  }
}

data "aws_iam_openid_connect_provider" "github_actions" {
  count = local.github_actions_enabled && !local.is_prod ? 1 : 0
  url   = "https://token.actions.githubusercontent.com"
}

locals {
  oidc_provider_arn = local.is_prod ? (
    local.github_actions_enabled ? aws_iam_openid_connect_provider.github_actions[0].arn : ""
  ) : (
    local.github_actions_enabled ? data.aws_iam_openid_connect_provider.github_actions[0].arn : ""
  )
}

data "aws_iam_policy_document" "github_actions_assume_role" {
  count = local.github_actions_enabled ? 1 : 0

  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = [local.oidc_provider_arn]
    }

    condition {
      test     = "StringEquals"
      variable = "token.actions.githubusercontent.com:aud"
      values   = ["sts.amazonaws.com"]
    }

    condition {
      test     = "StringEquals"
      variable = "token.actions.githubusercontent.com:sub"
      values = [
        for branch in var.github_actions_deploy_branches :
        "repo:${var.github_actions_repository}:ref:refs/heads/${branch}"
      ]
    }
  }
}

resource "aws_iam_role" "github_actions_deploy" {
  count = local.github_actions_enabled ? 1 : 0

  name               = "${local.name_prefix}-github-actions-deploy"
  assume_role_policy = data.aws_iam_policy_document.github_actions_assume_role[0].json

  tags = {
    Name = "${local.name_prefix}-github-actions-deploy"
  }
}

data "aws_iam_policy_document" "github_actions_deploy" {
  count = local.github_actions_enabled ? 1 : 0

  statement {
    sid       = "EcrLogin"
    effect    = "Allow"
    actions   = ["ecr:GetAuthorizationToken"]
    resources = ["*"]
  }

  statement {
    sid    = "EcrPush"
    effect = "Allow"
    actions = [
      "ecr:BatchCheckLayerAvailability",
      "ecr:BatchGetImage",
      "ecr:CompleteLayerUpload",
      "ecr:InitiateLayerUpload",
      "ecr:PutImage",
      "ecr:UploadLayerPart",
    ]
    resources = [
      local.ecr_api_arn,
      local.ecr_dashboard_arn,
    ]
  }

  statement {
    sid    = "EcsDeploy"
    effect = "Allow"
    actions = [
      "ecs:DescribeServices",
      "ecs:UpdateService",
    ]
    resources = ["*"]
  }
}

resource "aws_iam_role_policy" "github_actions_deploy" {
  count = local.github_actions_enabled ? 1 : 0

  name   = "github-actions-deploy"
  role   = aws_iam_role.github_actions_deploy[0].id
  policy = data.aws_iam_policy_document.github_actions_deploy[0].json
}
