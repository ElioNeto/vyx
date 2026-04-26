#!/usr/bin/env python3
"""Simple test runner without pytest."""

import asyncio
import os
import sys

# Add src to path so imports are consistent
script_dir = os.path.dirname(os.path.abspath(__file__))
src_dir = os.path.join(script_dir, 'src')
if src_dir not in sys.path:
    sys.path.insert(0, src_dir)

# Mock pytest.mark.asyncio
import unittest.mock
sys.modules['pytest'] = unittest.mock.MagicMock()
sys.modules['pytest.asyncio'] = unittest.mock.MagicMock()


def run_tests():
    """Run all tests."""
    failed = []
    passed = []
    
    # Test context.py
    print("=" * 60)
    print("Running context tests...")
    print("=" * 60)
    
    # Use consistent imports
    from vyx.context import get_correlation_id, set_correlation_id, clear_correlation_id
    
    # Test 1: get_correlation_id returns empty when not in context
    clear_correlation_id()
    result = get_correlation_id()
    if result == '':
        passed.append("test_get_correlation_id_returns_empty_when_not_in_context")
    else:
        failed.append(f"test_get_correlation_id_returns_empty_when_not_in_context: expected '', got {result!r}")
    
    # Test 2: set_and_get
    set_correlation_id('test-correlation-123')
    result = get_correlation_id()
    if result == 'test-correlation-123':
        passed.append("test_set_and_get_correlation_id")
    else:
        failed.append(f"test_set_and_get_correlation_id: expected 'test-correlation-123', got {result!r}")
    
    # Test 3: clear
    clear_correlation_id()
    result = get_correlation_id()
    if result == '':
        passed.append("test_clear_correlation_id")
    else:
        failed.append(f"test_clear_correlation_id: expected '', got {result!r}")
    
    # Test 4: no leak after clearing
    set_correlation_id('should-not-leak')
    clear_correlation_id()
    result = get_correlation_id()
    if result == '':
        passed.append("test_context_not_leaked_after_clearing")
    else:
        failed.append(f"test_context_not_leaked_after_clearing: expected '', got {result!r}")
    
    # Test 5: thread isolation
    import threading
    
    result = []
    def thread_worker():
        from vyx.context import set_correlation_id, get_correlation_id
        set_correlation_id('thread-id')
        result.append(get_correlation_id())
    
    thread = threading.Thread(target=thread_worker)
    thread.start()
    thread.join()
    
    # Main thread context unchanged
    from vyx.context import get_correlation_id as gc
    main_ctx = gc()
    thread_ctx = result[0] if result else ''
    
    if main_ctx == '' and thread_ctx == 'thread-id':
        passed.append("test_context_in_thread")
    else:
        failed.append(f"test_context_in_thread: main={main_ctx!r}, thread={thread_ctx!r}")
    
    print(f"\nPassed: {len(passed)}/{len(passed)+len(failed)}")
    for name in passed:
        print(f"  ✓ {name}")
    
    if failed:
        print(f"\nFailed: {len(failed)}/{len(passed)+len(failed)}")
        for name in failed:
            print(f"  ✗ {name}")
        return 1
    
    # Integration tests
    print("\n" + "=" * 60)
    print("Running integration tests...")
    print("=" * 60)
    
    from vyx.dispatch import Dispatcher, IPCPayload, WorkerResponse
    from vyx.context import set_correlation_id as sc, get_correlation_id as gc, clear_correlation_id as cc
    
    passed = []
    failed = []
    
    # Test dispatcher echoes correlation_id
    cc()
    dispatcher = Dispatcher('test-worker')
    
    def handler(req):
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
    
    if response['correlation_id'] == 'test-echo-123':
        passed.append("test_dispatcher_echoes_correlation_id_in_response")
    else:
        failed.append(f"test_dispatcher_echoes_correlation_id: expected 'test-echo-123', got {response['correlation_id']!r}")
    
    # Test concurrent requests (sequential in same thread - but each dispatch should work)
    cc()
    dispatcher = Dispatcher('test-worker')
    
    def handler2(req):
        # Use direct import to ensure consistency
        from vyx.context import get_correlation_id as gc2
        return {'correlation_id': gc2()}
    
    dispatcher.add_route('GET', '/api/test', handler2)
    
    results = []
    for i in range(1, 4):
        payload = IPCPayload({
            'method': 'GET',
            'path': '/api/test',
            'headers': {},
            'query': {},
            'params': {},
            'body': None,
            'claims': None,
            'correlation_id': f'req-{i:03d}',
        })
        response = dispatcher.dispatch(payload)
        results.append(response['body']['correlation_id'])
    
    if results == ['req-001', 'req-002', 'req-003']:
        passed.append("test_dispatcher_handles_concurrent_requests")
    else:
        failed.append(f"test_dispatcher_handles_concurrent_requests: expected ['req-001', 'req-002', 'req-003'], got {results}")
    
    # Test 404
    cc()
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
    
    if response['status_code'] == 404 and response['correlation_id'] == 'test-404':
        passed.append("test_dispatcher_returns_404_for_unknown_route")
    else:
        failed.append(f"test_dispatcher_returns_404: status={response['status_code']}, correlation_id={response['correlation_id']!r}")
    
    # Test context cleared after dispatch
    cc()
    dispatcher = Dispatcher('test-worker')
    
    def handler3(req):
        from vyx.context import get_correlation_id as gc3
        return {'correlation_id': gc3()}
    
    dispatcher.add_route('GET', '/api/test', handler3)
    
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
    
    if gc() == '':
        passed.append("test_context_is_cleared_after_dispatch")
    else:
        failed.append(f"test_context_is_cleared_after_dispatch: expected '', got {gc()!r}")
    
    # Test handler can access correlation_id
    cc()
    dispatcher = Dispatcher('test-worker')
    
    captured = []
    def handler4(req):
        from vyx.context import get_correlation_id as gc4
        cid = gc4()
        captured.append(cid)
        return {'correlation_id': cid}
    
    dispatcher.add_route('GET', '/api/users', handler4)
    
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
    
    if captured[0] == 'handler-access-123' and response['body']['correlation_id'] == 'handler-access-123':
        passed.append("test_handler_can_access_correlation_id")
    else:
        failed.append(f"test_handler_can_access_correlation_id: captured={captured[0]!r}, response={response['body']['correlation_id']!r}")
    
    print(f"\nPassed: {len(passed)}/{len(passed)+len(failed)}")
    for name in passed:
        print(f"  ✓ {name}")
    
    if failed:
        print(f"\nFailed: {len(failed)}/{len(passed)+len(failed)}")
        for name in failed:
            print(f"  ✗ {name}")
        return 1
    
    print("\n✓ All tests passed!")
    return 0


if __name__ == '__main__':
    sys.exit(run_tests())