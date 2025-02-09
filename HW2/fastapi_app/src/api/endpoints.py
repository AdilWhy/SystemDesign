from fastapi import APIRouter, Depends, HTTPException, Header, Form
from sqlmodel import Session, select
from datetime import datetime, timedelta, timezone
from ..models.user import User
from ..database import get_cached_user, get_session
from typing import Optional
import uuid
import time
from fastapi import Form
from sqlalchemy import text

router = APIRouter()


@router.post("/token/")
async def generate_token(
    client_id: str = Form(...),
    client_secret: str = Form(...),
    scope: str = Form(...),
    grant_type: str = Form(...),
    session: Session = Depends(get_session)
):
    if grant_type != "client_credentials":
        raise HTTPException(status_code=400, detail="Incorrect grant type")

    user = get_cached_user(client_id)
    if not user or user["client_secret"] != client_secret:
        raise HTTPException(
            status_code=400, detail="Incorrect client credentials")

    if scope not in user["scope"]:
        raise HTTPException(status_code=400, detail="Wrong scope")

    # Check existing token
    current_time = datetime.now(timezone.utc)
    existing_token = session.execute(
        text("""
            SELECT access_token, expiration_timeМ
            FROM public.token 
            WHERE client_id = :client_id AND access_scope = :scope
        """),
        {"client_id": client_id, "scope": scope}
    ).first()

    if existing_token:
        total_duration = timedelta(hours=2)
        remaining_time = existing_token.expiration_time - current_time
        if remaining_time > (total_duration / 2):
            # Return existing token if more than 50% time remains
            return {
                "access_token": existing_token.access_token,
                "token_type": "bearer",
                "expires_in": int(remaining_time.total_seconds()),
                "scope": scope
            }

    # Generate new token if needed
    token = str(uuid.uuid4())
    expiration_time = current_time + timedelta(hours=2)

    # Delete existing token if any
    session.execute(
        text("""
            DELETE FROM public.token 
            WHERE client_id = :client_id AND access_scope = :scope
        """),
        {"client_id": client_id, "scope": scope}
    )

    # Insert new token
    session.execute(
        text("""
            INSERT INTO public.token (client_id, access_scope, access_token, expiration_time)
            VALUES (:client_id, :scope, :token, :expiration_time)
        """),
        {
            "client_id": client_id,
            "scope": scope,
            "token": token,
            "expiration_time": expiration_time
        }
    )

    session.commit()

    return {
        "access_token": token,
        "token_type": "bearer",
        "expires_in": 7200,
        "scope": scope
    }


@router.get("/check/")
async def check_token(
    authorization: Optional[str] = Header(None),
    session: Session = Depends(get_session)
):
    if not authorization or not authorization.startswith("Bearer "):
        raise HTTPException(
            status_code=400,
            detail="Missing or invalid Authorization header"
        )

    token = authorization.split(" ")[1]

    # Check token in database
    result = session.execute(
        text("""
            SELECT client_id, access_scope, expiration_time 
            FROM public.token 
            WHERE access_token = :token
        """),
        {"token": token}
    ).first()

    if not result:
        raise HTTPException(
            status_code=400,
            detail="Invalid token"
        )

    current_time = datetime.now(timezone.utc)
    if result.expiration_time < current_time:
        raise HTTPException(
            status_code=400,
            detail="Token expired"
        )

    return {
        "client_id": result.client_id,
        "scope": result.access_scope
    }
