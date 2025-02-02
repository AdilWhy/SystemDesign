# language: python
import os
import json
import uuid
import datetime
from threading import Lock

from fastapi import FastAPI, Form, Header, HTTPException, status
from fastapi.responses import JSONResponse
import psycopg2
from psycopg2 import pool
from dotenv import load_dotenv

load_dotenv()

app = FastAPI()

# In-memory caches
users = {}  # { client_id: { "client_secret": str, "scopes": list, "tokens": { scope: token } } }
tokens = {}  # { token: { "client_id": str, "access_scope": str, "expiration_time": datetime } }
cache_lock = Lock()

# Initialize DB connection pool
DATABASE_URL = os.getenv("DATABASE_URL")
db_pool = psycopg2.pool.SimpleConnectionPool(1, 10, dsn=DATABASE_URL)


def init_users():
    """Load users.json and insert them into the DB."""
    try:
        with open("users.json", "r") as f:
            user_list = json.load(f)
    except Exception as e:
        raise Exception(f"Error reading users.json: {e}")

    conn = db_pool.getconn()
    try:
        with conn:
            with conn.cursor() as cur:
                for user in user_list:
                    cur.execute(
                        """
                        INSERT INTO public."user"(client_id, client_secret, scope)
                        VALUES (%s, %s, %s)
                        ON CONFLICT (client_id) DO NOTHING
                        """,
                        (user["client_id"], user["client_secret"], user["scope"])
                    )
    finally:
        db_pool.putconn(conn)


def load_all_users():
    """Load all users from DB into in-memory cache."""
    conn = db_pool.getconn()
    try:
        with conn:
            with conn.cursor() as cur:
                cur.execute(
                    'SELECT client_id, client_secret, scope FROM public."user"')
                rows = cur.fetchall()
                with cache_lock:
                    for row in rows:
                        cid, secret, scope = row
                        # Adjust if scope is stored as comma-separated string
                        scope_list = scope if isinstance(
                            scope, list) else scope.split(',')
                        users[cid] = {"client_secret": secret,
                                      "scopes": scope_list, "tokens": {}}
    finally:
        db_pool.putconn(conn)


def get_token_from_db(client_id, scope):
    """Retrieve a valid token from DB if exists."""
    conn = db_pool.getconn()
    token_val = None
    try:
        with conn:
            with conn.cursor() as cur:
                cur.execute(
                    "SELECT access_token, expiration_time FROM token WHERE client_id=%s AND access_scope=%s",
                    (client_id, scope)
                )
                row = cur.fetchone()
                if row:
                    token_val, exp_time = row
                    if exp_time < datetime.datetime.now():
                        cur.execute(
                            "DELETE FROM token WHERE access_token=%s", (token_val,))
                        token_val = None
    finally:
        db_pool.putconn(conn)
    return token_val


def add_token(client_id, scope):
    """Get an active token or generate a new one."""
    with cache_lock:
        user = users.get(client_id)
        if user and scope in user["tokens"] and user["tokens"][scope]:
            return user["tokens"][scope]
    token_val = get_token_from_db(client_id, scope)
    if token_val:
        with cache_lock:
            users[client_id]["tokens"][scope] = token_val
        return token_val

    token_val = str(uuid.uuid4())
    exp_time = datetime.datetime.now() + datetime.timedelta(seconds=7200)
    conn = db_pool.getconn()
    try:
        with conn:
            with conn.cursor() as cur:
                cur.execute(
                    "INSERT INTO token(client_id, access_scope, expiration_time, access_token) VALUES (%s, %s, %s, %s) RETURNING access_token",
                    (client_id, scope, exp_time, token_val)
                )
                row = cur.fetchone()
                if row:
                    token_val = row[0]
    finally:
        db_pool.putconn(conn)

    with cache_lock:
        users[client_id]["tokens"][scope] = token_val
        tokens[token_val] = {"client_id": client_id,
                             "access_scope": scope, "expiration_time": exp_time}
    return token_val


def check_token(token_val):
    """Validate the token and return client_id and scope if valid."""
    with cache_lock:
        if token_val in tokens:
            info = tokens[token_val]
            if info["expiration_time"] > datetime.datetime.now():
                return info["client_id"], info["access_scope"]
            else:
                del tokens[token_val]
                return None, "token expired"
    conn = db_pool.getconn()
    try:
        with conn:
            with conn.cursor() as cur:
                cur.execute(
                    "SELECT client_id, access_scope, expiration_time FROM token WHERE access_token=%s",
                    (token_val,)
                )
                row = cur.fetchone()
                if not row:
                    return None, "nonexistent token"
                client_id, scope, exp_time = row
                if exp_time < datetime.datetime.now():
                    return None, "token expired"
                with cache_lock:
                    tokens[token_val] = {
                        "client_id": client_id, "access_scope": scope, "expiration_time": exp_time}
                    if client_id in users:
                        users[client_id]["tokens"][scope] = token_val
                return client_id, scope
    finally:
        db_pool.putconn(conn)


@app.post("/token/")
async def token_endpoint(
    client_id: str = Form(...),
    scope: str = Form(...),
    client_secret: str = Form(...),
    grant_type: str = Form(...)
):
    if grant_type != "client_credentials":
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST, detail="Incorrect grant type")
    with cache_lock:
        user = users.get(client_id)
    if not user or user["client_secret"] != client_secret:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST,
                            detail="Incorrect client credentials")
    if scope not in user["scopes"]:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST, detail="Wrong scope")

    token_val = add_token(client_id, scope)
    if not token_val:
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR, detail="Error generating token")
    return JSONResponse(status_code=status.HTTP_200_OK, content={
        "access_token": token_val,
        "expires_in": 7200,
        "refresh_token": "",
        "scope": scope,
        "security_level": "normal",
        "token_type": "Bearer"
    })


@app.get("/check/")
async def check_endpoint(Authorization: str = Header(None)):
    if not Authorization:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST,
                            detail="Missing 'Authorization' header")
    parts = Authorization.split(" ")
    if len(parts) != 2 or parts[0] != "Bearer":
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST,
                            detail="Incorrect 'Authorization' header")
    token_val = parts[1]
    client_id, result = check_token(token_val)
    if client_id is None:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST, detail=result)
    return JSONResponse(status_code=status.HTTP_200_OK, content={"client_id": client_id, "scope": result})

if __name__ == '__main__':
    # Initialize users in DB and load into in-memory cache
    init_users()
    load_all_users()
    import uvicorn
    port = int(os.getenv("APP_PORT", "8000"))
    uvicorn.run(app, host="0.0.0.0", port=port)
