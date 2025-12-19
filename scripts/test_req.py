import urllib.request
import urllib.error
import json

token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJtcGMtaW5mcmEiLCJzdWIiOiJzeXN0ZW0tdGVzdCIsImV4cCI6MTc2NjE5ODE5MCwibmJmIjoxNzY2MTExNzkwLCJpYXQiOjE3NjYxMTE3OTAsInRlbmFudF9pZCI6InRlc3QtdGVuYW50IiwicGVybWlzc2lvbnMiOlsiYWRtaW4iXSwiYXBwX2lkIjoic3lzdGVtLXRlc3QifQ.9ZSDiIUNxguJearfBP69aHAE4whuVQNNk5PLR2nJyG0"
url = "http://localhost:8080/api/v1/infra/keys"
req = urllib.request.Request(url)
req.add_header("Authorization", f"Bearer {token}")
req.add_header("Content-Type", "application/json")

try:
    with urllib.request.urlopen(req) as response:
        print(f"Status: {response.status}")
        print(f"Body: {response.read().decode('utf-8')}")
except urllib.error.HTTPError as e:
    print(f"Status: {e.code}")
    print(f"Body: {e.read().decode('utf-8')}")
except Exception as e:
    print(f"Error: {e}")
