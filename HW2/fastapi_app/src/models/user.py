from sqlmodel import SQLModel, Field
from typing import List, Optional

class User(SQLModel, table=True):
    id: Optional[int] = Field(default=None, primary_key=True)
    client_secret: str
    scopes: str
    tokens: str