"""集中配置，环境变量驱动."""
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """运行时配置.

    全部字段都可通过同名大写环境变量覆盖。严禁在源码中硬编码密钥；
    生产中由 ExternalSecret 注入。
    """

    service_name: str = "member"
    version: str = "0.1.0"
    db_dsn: str = "postgresql+asyncpg://neko:neko@localhost:5432/nekocafe"
    otel_exporter_otlp_endpoint: str = "localhost:4317"
    otel_deployment_env: str = "dev"
    log_level: str = "INFO"

    model_config = SettingsConfigDict(env_file=".env", env_file_encoding="utf-8")


settings = Settings()
