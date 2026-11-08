"""Async HTTP example for the supported gateway contract (Python 3.11+)."""

import argparse
import asyncio
from dataclasses import dataclass
import json
import os
import sys
import time

import httpx


class GatewayFailure(Exception):
    def __init__(self, code, request_id=None, partial=False):
        super().__init__(code)
        self.code, self.request_id, self.partial = code, request_id, partial


@dataclass
class Completion:
    model: str
    text: str
    finish_reason: str
    request_id: str


def error_code(payload):
    error = payload.get("error") if isinstance(payload, dict) else None
    return error.get("code", "unknown_gateway_error") if isinstance(error, dict) else "upstream_error"


async def events(response):
    data = []
    size = 0
    async for line in response.aiter_lines():
        if line == "":
            if data:
                yield "\n".join(data)
                data, size = [], 0
        elif line.startswith("data:"):
            value = line[5:]
            if value.startswith(" "):
                value = value[1:]
            size += len(value.encode("utf-8"))
            if size > 128 * 1024:
                raise GatewayFailure("oversized_stream_event")
            data.append(value)
    if data:
        raise GatewayFailure("incomplete_stream")


async def generate(base_url, token, model, prompt, *, stream=False, max_tokens=128,
                   total_timeout=300, transport=None):
    partial = []
    request_id = None
    timeout = httpx.Timeout(connect=5, write=10, pool=5, read=300)
    transport = transport or httpx.AsyncHTTPTransport(retries=0)
    try:
        async with asyncio.timeout(total_timeout):
            async with httpx.AsyncClient(transport=transport, timeout=timeout,
                                        follow_redirects=False, trust_env=False) as client:
                async with client.stream("POST", base_url.rstrip("/") + "/v1/chat/completions",
                                         headers={"Authorization": "Bearer " + token},
                                         json={"model": model, "messages": [{"role": "user", "content": prompt}],
                                               "stream": stream, "max_tokens": max_tokens}) as response:
                    request_id = response.headers.get("x-request-id")
                    if response.status_code != 200:
                        body = await response.aread()
                        try:
                            code = error_code(json.loads(body))
                        except ValueError:
                            code = "invalid_error_response"
                        raise GatewayFailure(code, request_id)
                    if not stream:
                        payload = json.loads(await response.aread())
                        choice = payload["choices"][0]
                        if (payload.get("model") != model or len(payload["choices"]) != 1
                                or choice["message"].get("role") != "assistant"
                                or not isinstance(choice["message"].get("content"), str)
                                or choice.get("finish_reason") not in ("stop", "length")):
                            raise GatewayFailure("invalid_completion", request_id)
                        return Completion(model, choice["message"]["content"], choice["finish_reason"], request_id)
                    if not response.headers.get("content-type", "").startswith("text/event-stream"):
                        raise GatewayFailure("invalid_stream_type", request_id)
                    finish = None
                    done = False
                    async for data in events(response):
                        if done:
                            raise GatewayFailure("data_after_done", request_id, bool(partial))
                        if data == "[DONE]":
                            if finish is None:
                                raise GatewayFailure("done_without_finish", request_id, bool(partial))
                            done = True
                            continue
                        payload = json.loads(data)
                        if "error" in payload:
                            raise GatewayFailure(error_code(payload), request_id, bool(partial))
                        if finish is not None or payload.get("model") != model or len(payload["choices"]) != 1:
                            raise GatewayFailure("invalid_stream", request_id, bool(partial))
                        choice = payload["choices"][0]
                        text = choice["delta"].get("content", "")
                        if not isinstance(text, str):
                            raise GatewayFailure("invalid_stream", request_id, bool(partial))
                        if text:
                            partial.append(text)
                        if sum(len(piece) for piece in partial) > 1024 * 1024:
                            raise GatewayFailure("output_limit", request_id, True)
                        finish = choice.get("finish_reason")
                        if finish is not None and finish not in ("stop", "length"):
                            raise GatewayFailure("invalid_finish_reason", request_id, bool(partial))
                    if not done:
                        raise GatewayFailure("incomplete_stream", request_id, bool(partial))
                    return Completion(model, "".join(partial), finish, request_id)
    except GatewayFailure:
        raise
    except TimeoutError as exc:
        raise GatewayFailure("client_total_timeout", request_id, bool(partial)) from exc
    except (httpx.HTTPError, ValueError, KeyError, TypeError, IndexError) as exc:
        raise GatewayFailure("incomplete_or_invalid_response", request_id, bool(partial)) from exc


async def main(args):
    token = os.environ.get("AIGATEWAY_TOKEN", "")
    if not token:
        raise GatewayFailure("missing_token_environment")
    started = time.monotonic()
    result = await generate(args.base_url, token, args.model, args.prompt,
                            stream=args.stream, max_tokens=args.max_tokens)
    if args.summary_only:
        print(json.dumps({"model": result.model, "finish_reason": result.finish_reason,
                          "characters": len(result.text), "request_id": result.request_id,
                          "elapsed_seconds": round(time.monotonic() - started, 3)}))
    else:
        print(result.text)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base-url", default="http://127.0.0.1:8787")
    parser.add_argument("--model", required=True)
    parser.add_argument("--prompt", default="用一句话说明 CPU 和 GPU 的区别。")
    parser.add_argument("--max-tokens", type=int, default=128)
    parser.add_argument("--stream", action="store_true")
    parser.add_argument("--summary-only", action="store_true")
    try:
        asyncio.run(main(parser.parse_args()))
    except GatewayFailure as failure:
        print(json.dumps({"error": failure.code, "request_id": failure.request_id,
                          "partial": failure.partial}), file=sys.stderr)
        sys.exit(1)
    except KeyboardInterrupt:
        sys.exit(130)
