#!/usr/bin/env bash
set -euo pipefail

# Build and push Docker images to ECR.
#
# Usage:
#   ./scripts/deploy-images.sh              # build and push API + Dashboard
#   ./scripts/deploy-images.sh api          # API only
#   ./scripts/deploy-images.sh dashboard    # Dashboard only
#   TAG=v1.2.3 ./scripts/deploy-images.sh   # custom tag (default: latest)

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(dirname "${SCRIPT_DIR}")"

AWS_REGION="${AWS_REGION:-eu-central-1}"
AWS_ACCOUNT=$(aws sts get-caller-identity --query Account --output text)
ECR_BASE="${AWS_ACCOUNT}.dkr.ecr.${AWS_REGION}.amazonaws.com"
TAG="${TAG:-latest}"
TARGET="${1:-all}"
DOMAIN="${TASKBOARD_DOMAIN:-taskboard.commitment-tracker-aiops-sandbox.site}"
CLUSTER="taskboard-prod-cluster"

echo "=== Image Deploy ==="
echo "  Account:  ${AWS_ACCOUNT}"
echo "  Region:   ${AWS_REGION}"
echo "  Tag:      ${TAG}"
echo "  Target:   ${TARGET}"
echo ""

# Login to ECR
echo "Logging in to ECR..."
aws ecr get-login-password --region "${AWS_REGION}" | \
  docker login --username AWS --password-stdin "${ECR_BASE}"
echo ""

build_and_push() {
  local ecr_name=$1   # ECR repo name (e.g., taskboard/api)
  local context=$2     # Docker build context path
  local repo="${ECR_BASE}/${ecr_name}"

  echo "--- Building ${ecr_name} ---"
  docker build --platform linux/amd64 -t "${repo}:${TAG}" "${context}"

  if [ "${TAG}" != "latest" ]; then
    docker tag "${repo}:${TAG}" "${repo}:latest"
  fi

  echo "--- Pushing ${ecr_name} ---"
  docker push "${repo}:${TAG}"
  if [ "${TAG}" != "latest" ]; then
    docker push "${repo}:latest"
  fi
  echo ""
}

# API
if [ "${TARGET}" = "all" ] || [ "${TARGET}" = "api" ]; then
  build_and_push "taskboard/api" "${ROOT_DIR}/taskboard-api"
fi

# Dashboard
if [ "${TARGET}" = "all" ] || [ "${TARGET}" = "dashboard" ]; then
  DASHBOARD_REPO="${ECR_BASE}/taskboard/dashboard"
  echo "--- Building dashboard ---"
  docker build --platform linux/amd64 --no-cache \
    --build-arg "NEXT_PUBLIC_TASKBOARD_API_URL=https://${DOMAIN}/api/v1" \
    -t "${DASHBOARD_REPO}:${TAG}" "${ROOT_DIR}/taskboard-app"

  if [ "${TAG}" != "latest" ]; then
    docker tag "${DASHBOARD_REPO}:${TAG}" "${DASHBOARD_REPO}:latest"
  fi

  echo "--- Pushing dashboard ---"
  docker push "${DASHBOARD_REPO}:${TAG}"
  if [ "${TAG}" != "latest" ]; then
    docker push "${DASHBOARD_REPO}:latest"
  fi
  echo ""
fi

# Force ECS to pick up new images
echo "--- Updating ECS services ---"

if [ "${TARGET}" = "all" ] || [ "${TARGET}" = "api" ]; then
  aws ecs update-service --cluster "${CLUSTER}" --service taskboard-prod-api \
    --force-new-deployment --region "${AWS_REGION}" --no-cli-pager > /dev/null
  echo "  API service redeploying"
fi

if [ "${TARGET}" = "all" ] || [ "${TARGET}" = "dashboard" ]; then
  aws ecs update-service --cluster "${CLUSTER}" --service taskboard-prod-dashboard \
    --force-new-deployment --region "${AWS_REGION}" --no-cli-pager > /dev/null
  echo "  Dashboard service redeploying"
fi

echo ""
echo "Done. ECS will roll out new tasks over the next few minutes."
echo "Monitor: aws ecs describe-services --cluster ${CLUSTER} --services taskboard-prod-api taskboard-prod-dashboard --query 'services[].deployments[].{status:status,running:runningCount,desired:desiredCount}' --region ${AWS_REGION}"
