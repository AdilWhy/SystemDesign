from locust import FastHttpUser, task, between
import json
from random import choice
from json import JSONDecodeError

with open("users.json", 'r') as f:
    users = json.loads(f.read())

class WebsiteUser(FastHttpUser):
    @task
    def index(self):
        user = choice(users)
        scope = choice(user["scope"])
        with self.client.post("token/", data={
                                                "client_id": user["client_id"],
                                                "client_secret": user["client_secret"],
                                                "scope": scope,
                                                "grant_type": "client_credentials",
                                            }) as resp:
            try:
                token = resp.json()["access_token"]
            except JSONDecodeError:
                print(resp.content)
            else:
                with self.client.get("check/", headers={"Authorization": "Bearer " + token}) as resp:
                    json = resp.json()
                    if json["client_id"] != user["client_id"] or json["scope"] != scope:
                        resp.failure()