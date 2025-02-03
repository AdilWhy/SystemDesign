import asyncio
from dotenv import load_dotenv
from sqlmodel import SQLModel, create_engine, Session
import os

load_dotenv()

DATABASE_URL = os.getenv("DATABASE_URL")
if not DATABASE_URL:
    raise ValueError("DATABASE_URL environment variable is not set")

engine = create_engine(DATABASE_URL)


def create_db_and_tables():
    SQLModel.metadata.create_all(engine)


async def init_db():
    loop = asyncio.get_event_loop()
    await loop.run_in_executor(None, create_db_and_tables)


def get_session() -> Session:
    return Session(engine)
