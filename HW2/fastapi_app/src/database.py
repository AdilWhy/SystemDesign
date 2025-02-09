import asyncio
from dotenv import load_dotenv
from sqlmodel import SQLModel, create_engine, Session
import os
from sqlalchemy import text
from typing import Dict, Any


load_dotenv()
user_cache: Dict[str, Any] = {}
token_cache: Dict[str, Any] = {}


DATABASE_URL = os.getenv("DATABASE_URL")
if not DATABASE_URL:
    raise ValueError("DATABASE_URL environment variable is not set")

engine = create_engine(
    DATABASE_URL,
    pool_size=50,           # Increased pool size
    max_overflow=100,       # Increased overflow
    pool_timeout=60,        # Increased timeout
    pool_recycle=3600,      # Recycle connections hourly
    pool_pre_ping=True,     # Health check connections
    echo=False
)


def cache_all_users():
    """Load all users into memory cache"""
    with Session(engine) as session:
        users = session.exec(
            text("SELECT * FROM public.user")
        ).all()
        for user in users:
            user_cache[user.client_id] = {
                "client_secret": user.client_secret,
                "scope": user.scope
            }


def get_cached_user(client_id: str) -> Dict[str, Any]:
    """Get user from cache with consistent structure"""
    return user_cache.get(client_id, {})


def create_db_and_tables():
    SQLModel.metadata.create_all(engine)
    cache_all_users()


async def init_db():
    loop = asyncio.get_event_loop()
    await loop.run_in_executor(None, create_db_and_tables)

def get_session() -> Session:
    session = Session(engine)
    try:
        yield session
        session.commit()
    except Exception:
        session.rollback()
        raise
    finally:
        session.close()
