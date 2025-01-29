from string import ascii_lowercase
from random import randint, choices, sample
import json, sys, psycopg2, os
from urllib.parse import urlparse

def gen_users(n):
    scopes = ["push_send", "execute", "observe"]
    def gen_str(n):
        return ''.join(choices(ascii_lowercase, k=n))

    def gen_user():
        return {"client_id": gen_str(30), "client_secret": gen_str(50), "scope": sample(scopes, randint(1, len(scopes)))}

    users = [gen_user() for i in range(n)]
    with open("users.json", 'w') as f:
        f.write(json.dumps(users))

def load_to_db():
    with open("users.json", 'r') as f:
        users = json.loads(f.read())
    try:
        from dotenv import load_dotenv
        load_dotenv()
    except:
        pass
    conn_url = os.getenv('DATABASE_URL')
    p = urlparse(conn_url)
    pg_connection_dict = {a
        'dbname': p.path[1:],
        'user': p.username,
        'password': p.password,
        'port': p.port,
        'host': p.hostname
    }
    try:
        conn = psycopg2.connect(**pg_connection_dict)
        with conn.cursor() as curs:
            curs.executemany("insert into public.user(client_id, client_secret, scope) values (%s, %s, %s)", users)
        conn.commit()
        conn.close()
    except:
        pass

if "gen" in sys.argv:
    gen_users(1000)
else:
    load_to_db()