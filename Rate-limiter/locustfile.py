from locust import FastHttpUser, task, between
import json
from random import choice
from json import JSONDecodeError

with open("users.json", 'r') as f:
    users = json.loads(f.read())


class WebsiteUser(FastHttpUser):
    wait_time = between(1, 2)  # Adding wait time between requests

    @task
    def index(self):
        user = choice(users)
        scope = choice(user["scope"])

        # Token request
        with self.client.post("token/", data={
            "client_id": user["client_id"],
            "client_secret": user["client_secret"],
            "scope": scope,
            "grant_type": "client_credentials",
        }, catch_response=True) as resp:
            # Check for rate limiting on token endpoint
            if resp.status_code == 429:
                resp.failure(
                    f"Rate limited on token endpoint: {resp.json().get('error', 'unknown')}")
                return

            try:
                token = resp.json()["access_token"]
            except JSONDecodeError:
                print(resp.content)
                resp.failure("Invalid JSON response")
            except KeyError:
                resp.failure(f"No access token in response: {resp.content}")
            else:
                # Check endpoint request
                with self.client.get("check/", headers={"Authorization": "Bearer " + token}, catch_response=True) as resp:
                    # Check for rate limiting on check endpoint
                    if resp.status_code == 429:
                        resp.failure(
                            f"Rate limited on check endpoint: {resp.json().get('error', 'unknown')}")
                        return

                    try:
                        json_resp = resp.json()
                        if json_resp.get("client_id") != user["client_id"] or json_resp.get("scope") != scope:
                            resp.failure(f"Invalid response data: {json_resp}")
                    except JSONDecodeError:
                        resp.failure(f"Invalid JSON response: {resp.content}")
