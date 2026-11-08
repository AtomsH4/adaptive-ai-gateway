"""Validate the API description and synthetic contract fixtures, not a live server."""

import json
from pathlib import Path

from jsonschema import Draft202012Validator, FormatChecker
from openapi_spec_validator import validate_spec


ROOT = Path(__file__).resolve().parents[1]


def local_references_only(value):
    if isinstance(value, dict):
        for key, item in value.items():
            if key == "$ref" and not item.startswith("#/"):
                raise ValueError("API contract references must be self-contained")
            local_references_only(item)
    elif isinstance(value, list):
        for item in value:
            local_references_only(item)


def sse_events(text):
    lines = text.replace("\r\n", "\n").replace("\r", "\n").split("\n")
    if lines and lines[-1] == "":
        lines.pop()
    events, data = [], []
    for line in lines:
        if not line:
            if data:
                events.append("\n".join(data))
                data = []
        elif line.startswith("data:"):
            value = line[5:]
            data.append(value[1:] if value.startswith(" ") else value)
    return events, bool(data)


def stream_outcome(text, validators):
    events, trailing = sse_events(text)
    finished = done = failed = False
    for event in events:
        if done or failed:
            return "invalid"
        if event == "[DONE]":
            if not finished:
                return "invalid"
            done = True
            continue
        try:
            payload = json.loads(event)
        except ValueError:
            return "invalid"
        if not isinstance(payload, dict):
            return "invalid"
        if "error" in payload:
            if finished or not validators["Error"].is_valid(payload):
                return "invalid"
            failed = True
            continue
        if finished or not validators["CompletionChunk"].is_valid(payload):
            return "invalid"
        finished = payload["choices"][0]["finish_reason"] is not None
    if trailing:
        return "invalid" if done or failed else "incomplete"
    if failed:
        return "failed"
    return "completed" if done else "incomplete"


def main():
    specification = json.loads((ROOT / "api/openapi.json").read_text())
    fixtures = json.loads((ROOT / "api/fixtures/cases.json").read_text())
    local_references_only(specification)
    validate_spec(specification)
    validators = {
        name: Draft202012Validator(
            {"$ref": "#/components/schemas/" + name, "components": specification["components"]},
            format_checker=FormatChecker(),
        )
        for name in specification["components"]["schemas"]
    }
    failures = []
    for case in fixtures["json_cases"]:
        actual = validators[case["schema"]].is_valid(case["payload"])
        if actual != case["valid"]:
            failures.append(case["name"])
    for case in fixtures["stream_cases"]:
        if stream_outcome(case["data"], validators) != case["expected"]:
            failures.append(case["name"])
    if failures:
        raise SystemExit("Contract fixture mismatch: " + ", ".join(failures))
    print(
        "OpenAPI and contract fixtures: passed "
        f"({len(fixtures['json_cases'])} JSON, {len(fixtures['stream_cases'])} SSE). "
        "No live inference endpoint was exercised."
    )


if __name__ == "__main__":
    main()
