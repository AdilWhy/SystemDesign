import os
import json
import uuid
import datetime
from threading import Lock

from flask import Flask, request, jsonify
import psycopg2
from psycopg2 import pool
from dotenv import load_dotenv

load_dotenv()

app = Flask(__name__)

# Global in-memory caches
users = {}    # { client_id: { "client_secret": str, "scopes": list, "tokens": { scope: token } } }
tokens = {}   # { token: { "client_id": str, "access_scope": str, "expiration_time": datetime } }
cache_lock = Lock()

# Initialize DB connection pool
DATABASE_URL = os.getenv("DATABASE_URL")
db_pool = psycopg2.pool.SimpleConnectionPool(
    1, 10, dsn=DATABASE_URL
)


def init_users():
    """Read users.json and insert them into the database."""
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
                    # Insert user if not exists using ON CONFLICT (adjust conflict target per your schema)
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
    """Load all users from DB into memory cache."""
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
                        # Assume scope is stored as an array in DB; if stored as text, adjust accordingly (e.g., json.loads)
                        users[cid] = {
                            "client_secret": secret,
                            "scopes": scope if isinstance(scope, list) else scope.split(','),
                            "tokens": {}
                        }
    finally:
        db_pool.putconn(conn)


def get_token_from_db(client_id, scope):
    """Attempt to get a valid token from DB for a client and scope."""
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
    """Return a valid token for a given client and scope; generate a new one if needed."""
    with cache_lock:
        user = users.get(client_id)
        if user and scope in user["tokens"] and user["tokens"][scope]:
            return user["tokens"][scope]

    token_val = get_token_from_db(client_id, scope)
    if token_val:
        with cache_lock:
            users[client_id]["tokens"][scope] = token_val
        return token_val

    # Generate a new token
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
        tokens[token_val] = {
            "client_id": client_id,
            "access_scope": scope,
            "expiration_time": exp_time
        }
    return token_val


def check_token(token_val):
    """Validate token. Return (client_id, scope) if valid, else (None, error_message)."""
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
                        "client_id": client_id,
                        "access_scope": scope,
                        "expiration_time": exp_time
                    }
                    if client_id in users:
                        users[client_id]["tokens"][scope] = token_val
                return client_id, scope
    finally:
        db_pool.putconn(conn)


@app.route('/token/', methods=['POST'])
def token_endpoint():
    client_id = request.form.get("client_id")
    scope = request.form.get("scope")
    client_secret = request.form.get("client_secret")
    grant_type = request.form.get("grant_type")

    if not all([client_id, scope, client_secret, grant_type]):
        return jsonify({"error": "Missing fields: client_id, scope, client_secret, grant_type"}), 400

    if grant_type != "client_credentials":
        return jsonify({"error": "Incorrect grant type"}), 400

    with cache_lock:
        user = users.get(client_id)

    if not user or user["client_secret"] != client_secret:
        return jsonify({"error": "Incorrect client credentials"}), 400

    if scope not in user["scopes"]:
        return jsonify({"error": "Wrong scope"}), 400

    token_val = add_token(client_id, scope)
    if not token_val:
        return jsonify({"error": "Internal server error"}), 500

    return jsonify({
        "access_token": token_val,
        "expires_in": 7200,
        "refresh_token": "",
        "scope": scope,
        "security_level": "normal",
        "token_type": "Bearer",
    }), 200


@app.route('/check/', methods=['GET'])
def check_endpoint():
    auth_header = request.headers.get("Authorization", "")
    parts = auth_header.split(" ")
    if len(parts) != 2 or parts[0] != "Bearer":
        return jsonify({"error": "Incorrect 'Authorization' header"}), 400

    token_val = parts[1]
    result = check_token(token_val)
    if result[0] is None:
        return jsonify({"error": result[1]}), 400
    client_id, scope = result
    return jsonify({"client_id": client_id, "scope": scope}), 200


if __name__ == '__main__':
    # Load users from file and database
    init_users()
    load_all_users()
    port = int(os.getenv("APP_PORT", "8000"))
    app.run(host="0.0.0.0", port=port)
