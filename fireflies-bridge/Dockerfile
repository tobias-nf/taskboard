FROM python:3.13-slim

WORKDIR /app

COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv

COPY pyproject.toml .
RUN uv pip install --system --no-cache \
    "fastapi>=0.115" \
    "uvicorn[standard]>=0.34" \
    "httpx>=0.28" \
    "anthropic>=0.52" \
    "pydantic>=2.11" \
    "pydantic-settings>=2.9"

COPY . .

EXPOSE 8000
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"]
