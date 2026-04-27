from dataclasses import dataclass
from typing import Any, Callable, Dict, Optional

from .context import (
    get_correlation_id,
    set_correlation_id,
    reset_correlation_id,
    clear_correlation_id,
)


@dataclass
class IPCPayload:
    """Incoming IPC request payload."""

    method: str
    path: str
    headers: dict
    query: dict
    params: dict
    body: Any
    claims: Any
    correlation_id: str

    def __init__(self, data: dict):
        self.method = data.get('method', '')
        self.path = data.get('path', '')
        self.headers = data.get('headers', {})
        self.query = data.get('query', {})
        self.params = data.get('params', {})
        self.body = data.get('body')
        self.claims = data.get('claims')
        self.correlation_id = data.get('correlation_id', '')


WorkerResponse = dict  # {"status_code": int, "headers": dict, "body": Any, "correlation_id": str}


class Dispatcher:
    """IPC dispatcher that routes requests to handlers."""

    def __init__(self, worker_id: str):
        self.worker_id = worker_id
        self.routes: Dict[tuple, Callable] = {}
        self.async_routes: Dict[tuple, Callable] = {}

    def add_route(self, method: str, path: str, handler: Callable) -> None:
        """Register a synchronous route handler."""
        self.routes[(method, path)] = handler

    def add_async_route(self, method: str, path: str, handler: Callable) -> None:
        """Register an asynchronous route handler."""
        self.async_routes[(method, path)] = handler

    def _match_route(self, method: str, path: str) -> Optional[Callable]:
        """Match a route handler."""
        if (method, path) in self.routes:
            return self.routes[(method, path)]
        if (method, path) in self.async_routes:
            return self.async_routes[(method, path)]
        return None

    def _build_request(self, payload: IPCPayload) -> dict:
        """Build request dict from IPCPayload."""
        return {
            'method': payload.method,
            'path': payload.path,
            'headers': payload.headers,
            'query': payload.query,
            'params': payload.params,
            'body': payload.body,
            'claims': payload.claims,
        }

    def dispatch(self, payload: IPCPayload) -> WorkerResponse:
        """Dispatch a synchronous request."""
        handler = self._match_route(payload.method, payload.path)
        correlation_id = payload.correlation_id

        if not handler:
            return {
                'status_code': 404,
                'headers': {},
                'body': {'error': 'route not found'},
                'correlation_id': correlation_id,
            }

        token = set_correlation_id(correlation_id)
        try:
            result = handler(self._build_request(payload))
            return {
                'status_code': 200,
                'headers': {'Content-Type': 'application/json'},
                'body': result,
                'correlation_id': correlation_id,
            }
        finally:
            reset_correlation_id(token)

    async def dispatch_async(self, payload: IPCPayload) -> WorkerResponse:
        """Dispatch an asynchronous request."""
        handler = self._match_route(payload.method, payload.path)
        correlation_id = payload.correlation_id

        if not handler:
            return {
                'status_code': 404,
                'headers': {},
                'body': {'error': 'route not found'},
                'correlation_id': correlation_id,
            }

        token = set_correlation_id(correlation_id)
        try:
            result = await handler(self._build_request(payload))
            return {
                'status_code': 200,
                'headers': {'Content-Type': 'application/json'},
                'body': result,
                'correlation_id': correlation_id,
            }
        finally:
            reset_correlation_id(token)