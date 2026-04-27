"""Integration tests for concurrent IPC requests with correlation ID."""

import asyncio
import pytest

from vyx.context import set_correlation_id, get_correlation_id, clear_correlation_id
from vyx.dispatch import Dispatcher, IPCPayload, WorkerResponse


def test_dispatcher_echoes_correlation_id_in_response():
    """Should echo correlation_id in response envelope."""
    dispatcher = Dispatcher('test-worker')
    
    def handler(req: dict) -> dict:
        return {'message': 'ok'}
    
    dispatcher.add_route('GET', '/api/test', handler)
    
    payload = IPCPayload({
        'method': 'GET',
        'path': '/api/test',
        'headers': {},
        'query': {},
        'params': {},
        'body': None,
        'claims': None,
        'correlation_id': 'test-echo-123',
    })
    
    response = dispatcher.dispatch(payload)
    
    assert response['correlation_id'] == 'test-echo-123'


def test_dispatcher_handles_concurrent_requests():
    """Should handle concurrent requests with isolated correlation IDs."""
    dispatcher = Dispatcher('test-worker')
    
    def handler(req: dict) -> dict:
        correlation_id = get_correlation_id()
        return {'correlation_id': correlation_id}
    
    dispatcher.add_route('GET', '/api/test', handler)
    
    payloads = [
        IPCPayload({
            'method': 'GET',
            'path': '/api/test',
            'headers': {},
            'query': {},
            'params': {},
            'body': None,
            'claims': None,
            'correlation_id': f'req-{i:03d}',
        })
        for i in range(1, 4)
    ]
    
    results = []
    for payload in payloads:
        response = dispatcher.dispatch(payload)
        results.append(response['body']['correlation_id'])
    
    assert results == ['req-001', 'req-002', 'req-003']


def test_dispatcher_returns_404_for_unknown_route():
    """Should return 404 for unknown routes."""
    dispatcher = Dispatcher('test-worker')
    
    payload = IPCPayload({
        'method': 'GET',
        'path': '/api/unknown',
        'headers': {},
        'query': {},
        'params': {},
        'body': None,
        'claims': None,
        'correlation_id': 'test-404',
    })
    
    response = dispatcher.dispatch(payload)
    
    assert response['status_code'] == 404
    assert response['correlation_id'] == 'test-404'


@pytest.mark.asyncio
async def test_async_dispatcher_handles_concurrent_requests():
    """Should handle concurrent async requests with isolated correlation IDs."""
    dispatcher = Dispatcher('test-worker')
    
    async def async_handler(req: dict) -> dict:
        await asyncio.sleep(0.01)  # Simulate async work
        return {'correlation_id': get_correlation_id()}
    
    dispatcher.add_async_route('GET', '/api/async', async_handler)
    
    payloads = [
        IPCPayload({
            'method': 'GET',
            'path': '/api/async',
            'headers': {},
            'query': {},
            'params': {},
            'body': None,
            'claims': None,
            'correlation_id': f'async-{i:03d}',
        })
        for i in range(1, 4)
    ]
    
    results = []
    for payload in payloads:
        response = await dispatcher.dispatch_async(payload)
        results.append(response['body']['correlation_id'])
    
    assert results == ['async-001', 'async-002', 'async-003']


def test_context_is_cleared_after_dispatch():
    """Should clear context after dispatch completes."""
    dispatcher = Dispatcher('test-worker')
    
    def handler(req: dict) -> dict:
        return {'correlation_id': get_correlation_id()}
    
    dispatcher.add_route('GET', '/api/test', handler)
    
    payload = IPCPayload({
        'method': 'GET',
        'path': '/api/test',
        'headers': {},
        'query': {},
        'params': {},
        'body': None,
        'claims': None,
        'correlation_id': 'clear-test',
    })
    
    dispatcher.dispatch(payload)
    
    # Context should be cleared after dispatch
    assert get_correlation_id() == ''


def test_handler_can_access_correlation_id():
    """Handler should be able to access correlation ID."""
    dispatcher = Dispatcher('test-worker')
    
    captured_id = []
    
    def handler(req: dict) -> dict:
        captured_id.append(get_correlation_id())
        return {'correlation_id': get_correlation_id()}
    
    dispatcher.add_route('GET', '/api/users', handler)
    
    payload = IPCPayload({
        'method': 'GET',
        'path': '/api/users',
        'headers': {},
        'query': {},
        'params': {},
        'body': None,
        'claims': None,
        'correlation_id': 'handler-access-123',
    })
    
    response = dispatcher.dispatch(payload)
    
    assert captured_id[0] == 'handler-access-123'
    assert response['body']['correlation_id'] == 'handler-access-123'