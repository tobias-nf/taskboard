from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    # Taskboard
    taskboard_url: str = "https://taskboard.nearintents.org/api/v1"
    taskboard_api_key: str  # hive_sk_taskboard-slack_<secret>

    # Slack
    slack_bot_token: str  # xoxb-...
    slack_signing_secret: str = ""  # Not needed in Socket Mode
    slack_app_token: str = ""  # xapp-... (Socket Mode only, for local dev)

    # Anthropic (for /task natural language agent)
    anthropic_api_key: str

    # Identity
    app_agent_id: str = "taskboard-slack"
    admin_slack_id: str = ""  # Fallback for unresolvable agents

    model_config = {"env_prefix": "", "env_file": ".env"}


settings = Settings()
