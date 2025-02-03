from fastapi import APIRouter, HTTPException, Depends
from sqlmodel import Session, select
from ..models.user import User
from ..database import get_session
import uuid
import time

router = APIRouter()

@router.post("/token/")
async def generate_token(client_id: str, scope: str, client_secret: str, grant_type: str, session: Session = Depends(get_session)):
    if grant_type != "client_credentials":
        raise HTTPException(status_code=400, detail="Incorrect grant type")

    user = session.exec(select(User).where(User.client_id == client_id)).first()
    if not user or user.client_secret != client_secret:
        raise HTTPException(status_code=400, detail="Incorrect client credentials")

    if scope not in user.scopes:
        raise HTTPException(status_code=400, detail="Wrong scope")

    token = str(uuid.uuid4())
    expiration_time = time.time() + 7200  # Token valid for 2 hours

    user.tokens.append(token)
    session.add(user)
    session.commit()

    return {
        "access_token": token,
        "expires_in": 7200,
        "refresh_token": "",
        "scope": scope,
        "security_level": "normal",
        "token_type": "Bearer",
    }

@router.get("/check/")
async def check_token(authorization: str, session: Session = Depends(get_session)):
    if not authorization.startswith("Bearer "):
        raise HTTPException(status_code=400, detail="Incorrect 'Authorization' header")

    token = authorization.split(" ")[1]
    user = session.exec(select(User).where(User.tokens.contains(token))).first()

    if not user:
        raise HTTPException(status_code=400, detail="Nonexistent token")

    return {
        "client_id": user.client_id,
        "scope": user.scopes,
    }