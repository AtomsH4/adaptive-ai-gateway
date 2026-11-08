"""Validate an explicitly running local gateway against the checked-in schema."""
import argparse
import json
import os
from pathlib import Path
import sys
import urllib.request

from jsonschema import Draft202012Validator, FormatChecker

sys.path.insert(0, str(Path(__file__).parent))
from check_contracts import stream_outcome

args = argparse.ArgumentParser()
args.add_argument("--base-url", default="http://127.0.0.1:8787")
args.add_argument("--model", required=True)
options = args.parse_args()
spec = json.loads((Path(__file__).resolve().parents[1] / "api/openapi.json").read_text())
validators = {name: Draft202012Validator({"$ref": "#/components/schemas/" + name, "components": spec["components"]}, format_checker=FormatChecker()) for name in spec["components"]["schemas"]}
token = os.environ["AIGATEWAY_TOKEN"]
def call(path, payload=None):
    body = None if payload is None else json.dumps(payload).encode()
    request = urllib.request.Request(options.base_url + path, body, {"Authorization": "Bearer " + token, "Content-Type": "application/json"})
    with urllib.request.urlopen(request, timeout=300) as response:
        assert response.headers.get("X-Request-ID")
        return response.read()
for path, schema in [("/healthz", "Health"), ("/readyz", "Ready"), ("/v1/models", "ModelList"), ("/admin/resources", "ResourceDiagnostics"), ("/admin/scheduler", "SchedulerDiagnostics")]:
    validators[schema].validate(json.loads(call(path)))
payload = {"model": options.model, "messages": [{"role": "user", "content": "Reply with one short sentence."}], "max_tokens": 32}
validators["ChatCompletion"].validate(json.loads(call("/v1/chat/completions", payload)))
payload["stream"] = True
assert stream_outcome(call("/v1/chat/completions", payload).decode(), validators) == "completed"
print("Live HTTP contract: passed (5 GET, 1 JSON completion, 1 SSE completion).")
