# Platform Infrastructure

Terraform for the shared AWS platform behind Taskboard and future services.

This directory keeps infrastructure in a single Terraform root module so
resource addresses stay stable while the ownership boundaries get cleaner.

## Current Layout

- `00-bootstrap.tf` — Terraform settings and AWS providers
- `01-inputs.tf` — shared inputs used by the current root module
- `10-foundation-network.tf` — VPC, subnets, routing, and security groups
- `11-foundation-data.tf` — shared Postgres
- `12-foundation-storage.tf` — shared S3 bucket and access policy
- `20-shared-routing.tf` — ALB, target groups, and listener rules
- `21-shared-edge.tf` — ACM and CloudFront
- `22-shared-dns.tf` — Route 53 zone and DNS records
- `30-taskboard-images.tf` — Taskboard ECR repositories
- `31-taskboard-services.tf` — ECS cluster, task definitions, and ECS services
- `32-taskboard-secrets.tf` — Secrets Manager and ECS secret access
- `33-platform-delivery.tf` — GitHub Actions OIDC deploy role
- `34-platform-observability.tf` — SNS alarms and CloudWatch alarms
- `35-platform-grafana.tf` — optional Amazon Managed Grafana
- `99-outputs.tf` — outputs consumed by humans and future stack splits

## Ownership Boundaries

- Foundation: `00-12`
- Shared edge/platform: `20-22`, `33`, `35`
- Taskboard runtime: `30-34`

## Workflow

1. Copy `secrets.auto.tfvars.example` to `secrets.auto.tfvars`.
2. Fill in the real values.
3. Run `terraform init` from this directory.
4. Run `terraform plan`.
5. Run `terraform apply`.

Preferred inputs:

- `taskboard_api_secrets`
- `github_actions_repository` / `github_actions_deploy_branch`
- `grafana_enabled` plus the `grafana_*` variables

## Secret And State Tradeoff

Terraform stores managed secret values in state. That is convenient for review
and change tracking, but it makes the state backend sensitive infrastructure.

Use an encrypted remote backend and tightly control access before managing
production secrets this way.

## Hygiene

- Do not commit filled `*.tfvars` files.
- Do not commit local state files.
- Do not commit generated plan files such as `tfplan` or `*.tfplan`.
