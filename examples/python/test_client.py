import asyncio
import json
from pathlib import Path
import sys
import unittest

import httpx

sys.path.insert(0, str(Path(__file__).parent))
from client import GatewayFailure, generate


class ClientContract(unittest.IsolatedAsyncioTestCase):
    async def call(self, body):
        transport = httpx.MockTransport(lambda _: httpx.Response(200, headers={"content-type": "text/event-stream"}, text=body))
        return await generate("http://fixture", "test", "model", "test", stream=True, transport=transport)

    async def test_finish_and_done_required(self):
        delta = {"model": "model", "choices": [{"delta": {"content": "部分"}, "finish_reason": None}]}
        finish = {"model": "model", "choices": [{"delta": {}, "finish_reason": "length"}]}
        result = await self.call("data: " + json.dumps(delta) + "\n\ndata: " + json.dumps(finish) + "\n\ndata: [DONE]\n\n")
        self.assertEqual(result.finish_reason, "length")
        for body in ("data: [DONE]\n\n", "data: " + json.dumps(delta) + "\n\n"):
            with self.assertRaises(GatewayFailure):
                await self.call(body)

    async def test_unknown_error_is_failure_even_after_partial_output(self):
        body = 'data: {"model":"model","choices":[{"delta":{"content":"partial"},"finish_reason":null}]}\n\ndata: {"error":{"code":"future_error"}}\n\n'
        with self.assertRaises(GatewayFailure) as raised:
            await self.call(body)
        self.assertEqual(raised.exception.code, "future_error")
        self.assertTrue(raised.exception.partial)

    async def test_outer_deadline_cancels_transport(self):
        closed = asyncio.Event()
        async def slow(_):
            try:
                await asyncio.sleep(100)
            finally:
                closed.set()
        with self.assertRaises(GatewayFailure) as raised:
            await generate("http://fixture", "test", "model", "test", total_timeout=0.01, transport=httpx.MockTransport(slow))
        self.assertEqual(raised.exception.code, "client_total_timeout")
        self.assertTrue(closed.is_set())
