from pydantic import BaseSettings

class Settings(BaseSettings):
    database_url: str
    app_port: int = 8000
    release: bool = False

    class Config:
        env_file = ".env"

settings = Settings()