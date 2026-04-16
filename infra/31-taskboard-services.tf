# --- ECS Cluster ---

resource "aws_ecs_cluster" "main" {
  name = "${local.name_prefix}-cluster"

  setting {
    name  = "containerInsights"
    value = "enabled"
  }

  tags = { Name = "${local.name_prefix}-cluster" }
}

# --- IAM Roles ---

data "aws_iam_policy_document" "ecs_task_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "ecs_execution" {
  name               = "${local.name_prefix}-ecs-execution"
  assume_role_policy = data.aws_iam_policy_document.ecs_task_assume.json
}

resource "aws_iam_role_policy_attachment" "ecs_execution" {
  role       = aws_iam_role.ecs_execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

resource "aws_iam_role" "api_task" {
  name               = "${local.name_prefix}-api-task"
  assume_role_policy = data.aws_iam_policy_document.ecs_task_assume.json
}

resource "aws_iam_role_policy" "api_s3_access" {
  name = "s3-access"
  role = aws_iam_role.api_task.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:GetObject",
          "s3:PutObject",
          "s3:DeleteObject",
          "s3:ListBucket"
        ]
        Resource = [
          aws_s3_bucket.main.arn,
          "${aws_s3_bucket.main.arn}/*"
        ]
      }
    ]
  })
}

resource "aws_iam_role" "dashboard_task" {
  name               = "${local.name_prefix}-dashboard-task"
  assume_role_policy = data.aws_iam_policy_document.ecs_task_assume.json
}

# --- CloudWatch Log Groups ---

resource "aws_cloudwatch_log_group" "api" {
  name              = "/ecs/${local.name_prefix}/api"
  retention_in_days = 30
  tags              = { Name = "${local.name_prefix}-api-logs" }
}

resource "aws_cloudwatch_log_group" "dashboard" {
  name              = "/ecs/${local.name_prefix}/dashboard"
  retention_in_days = 30
  tags              = { Name = "${local.name_prefix}-dashboard-logs" }
}

# --- Task Definitions ---

resource "aws_ecs_task_definition" "api" {
  family                   = "${local.name_prefix}-api"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = var.api_cpu
  memory                   = var.api_memory
  execution_role_arn       = aws_iam_role.ecs_execution.arn
  task_role_arn            = aws_iam_role.api_task.arn

  container_definitions = jsonencode([
    {
      name      = "api"
      image     = "${local.ecr_api_url}:latest${local.is_prod ? "" : "-dev"}"
      essential = true

      portMappings = [
        {
          containerPort = 4000
          protocol      = "tcp"
        }
      ]

      secrets = [
        { name = "TASKBOARD_ADMIN_API_KEY", valueFrom = "${aws_secretsmanager_secret.taskboard_api.arn}:TASKBOARD_ADMIN_API_KEY::" },
        { name = "GOOGLE_CLIENT_ID", valueFrom = "${aws_secretsmanager_secret.taskboard_api.arn}:GOOGLE_CLIENT_ID::" },
        { name = "GOOGLE_CLIENT_SECRET", valueFrom = "${aws_secretsmanager_secret.taskboard_api.arn}:GOOGLE_CLIENT_SECRET::" },
        { name = "SESSION_SECRET", valueFrom = "${aws_secretsmanager_secret.taskboard_api.arn}:SESSION_SECRET::" },
      ]

      environment = [
        { name = "PORT", value = "4000" },
        { name = "ENV", value = var.environment },
        { name = "DATABASE_URL", value = "postgres://hive:${var.db_password}@${aws_db_instance.main.endpoint}/hive?sslmode=require" },
        { name = "S3_BUCKET", value = aws_s3_bucket.main.id },
        { name = "S3_REGION", value = var.aws_region },
        { name = "CORS_ORIGIN", value = "https://${var.domain_name}" },
        { name = "GOOGLE_REDIRECT_URI", value = "https://${var.domain_name}/auth/google/callback" },
        { name = "FRONTEND_URL", value = "https://${var.domain_name}/app" },
        { name = "ALLOWED_EMAIL_DOMAIN", value = "near.foundation" },
      ]

      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"         = aws_cloudwatch_log_group.api.name
          "awslogs-region"        = var.aws_region
          "awslogs-stream-prefix" = "api"
        }
      }

      healthCheck = {
        command     = ["CMD-SHELL", "wget -q --spider http://localhost:4000/health || exit 1"]
        interval    = 30
        timeout     = 5
        retries     = 3
        startPeriod = 10
      }
    }
  ])

  tags = { Name = "${local.name_prefix}-api" }
}

resource "aws_ecs_task_definition" "dashboard" {
  family                   = "${local.name_prefix}-dashboard"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = var.dashboard_cpu
  memory                   = var.dashboard_memory
  execution_role_arn       = aws_iam_role.ecs_execution.arn
  task_role_arn            = aws_iam_role.dashboard_task.arn

  container_definitions = jsonencode([
    {
      name      = "dashboard"
      image     = "${local.ecr_dashboard_url}:latest${local.is_prod ? "" : "-dev"}"
      essential = true

      portMappings = [
        {
          containerPort = 4001
          protocol      = "tcp"
        }
      ]

      environment = [
        { name = "PORT", value = "4001" },
        { name = "NEXT_PUBLIC_TASKBOARD_API_URL", value = "https://${var.domain_name}/api/v1" },
      ]

      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"         = aws_cloudwatch_log_group.dashboard.name
          "awslogs-region"        = var.aws_region
          "awslogs-stream-prefix" = "dashboard"
        }
      }
    }
  ])

  tags = { Name = "${local.name_prefix}-dashboard" }
}

# --- ECS Services ---

resource "aws_ecs_service" "api" {
  name            = "${local.name_prefix}-api"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.api.arn
  desired_count   = 1
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = aws_subnet.private[*].id
    security_groups  = [aws_security_group.ecs.id]
    assign_public_ip = false
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.api.arn
    container_name   = "api"
    container_port   = 4000
  }

  depends_on = [aws_lb_listener.http]

  tags = { Name = "${local.name_prefix}-api-service" }
}

resource "aws_ecs_service" "dashboard" {
  name            = "${local.name_prefix}-dashboard"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.dashboard.arn
  desired_count   = 1
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = aws_subnet.private[*].id
    security_groups  = [aws_security_group.ecs.id]
    assign_public_ip = false
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.dashboard.arn
    container_name   = "dashboard"
    container_port   = 4001
  }

  depends_on = [aws_lb_listener.http]

  tags = { Name = "${local.name_prefix}-dashboard-service" }
}
