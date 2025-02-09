from sqlmodel import SQLModel, Field
from typing import List
from sqlalchemy import Column, String
from sqlalchemy.dialects.postgresql import ARRAY


class User(SQLModel, table=True):
    client_id: str = Field(sa_column=Column(String(50), primary_key=True))
    client_secret: str = Field(sa_column=Column(String))
    scope: List[str] = Field(default_factory=list,
                             sa_column=Column(ARRAY(String)))
