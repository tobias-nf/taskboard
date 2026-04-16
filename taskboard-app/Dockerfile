FROM node:22-alpine AS builder

RUN corepack enable && corepack prepare pnpm@latest --activate

WORKDIR /app
COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY . .
ARG NEXT_PUBLIC_TASKBOARD_API_URL=http://localhost:4000/api/v1
ARG NEXT_PUBLIC_TASKBOARD_AUTH_URL=http://localhost:4000/auth
ARG NEXT_PUBLIC_DEV_MODE=
ENV NEXT_PUBLIC_TASKBOARD_API_URL=$NEXT_PUBLIC_TASKBOARD_API_URL
ENV NEXT_PUBLIC_TASKBOARD_AUTH_URL=$NEXT_PUBLIC_TASKBOARD_AUTH_URL
ENV NEXT_PUBLIC_DEV_MODE=$NEXT_PUBLIC_DEV_MODE
RUN pnpm run build

FROM node:22-alpine
RUN corepack enable && corepack prepare pnpm@latest --activate

WORKDIR /app
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static

ENV PORT=4001
EXPOSE 4001
CMD ["node", "server.js"]
