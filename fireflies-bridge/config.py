from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    # Fireflies
    fireflies_api_key: str
    fireflies_webhook_secret: str = ""

    # Anthropic
    anthropic_api_key: str

    # Taskboard
    taskboard_url: str = "https://taskboard.nearintents.org/api/v1"
    taskboard_api_key: str  # hive_sk_fireflies-bridge_<secret>

    # Bridge identity
    bridge_agent_id: str = "fireflies-bridge"
    slack_app_agent_id: str = "taskboard-slack"  # Mentioned on drafts so the Slack app gets SSE events

    model_config = {"env_prefix": "", "env_file": ".env"}


settings = Settings()
