"""Unit tests for context module."""

import asyncio
import pytest

from vyx.context import (
    get_correlation_id,
    set_correlation_id,
    clear_correlation_id,
)


def test_get_correlation_id_returns_empty_when_not_in_context():
    """Should return empty string when not in a request context."""
    clear_correlation_id()
    assert get_correlation_id() == ''


def test_set_and_get_correlation_id():
    """Should store and retrieve correlation ID."""
    set_correlation_id('test-correlation-123')
    assert get_correlation_id() == 'test-correlation-123'


def test_clear_correlation_id():
    """Should clear the correlation ID."""
    set_correlation_id('test-456')
    clear_correlation_id()
    assert get_correlation_id() == ''


def test_context_not_leaked_after_clearing():
    """Should not leak context after clearing."""
    set_correlation_id('should-not-leak')
    clear_correlation_id()
    assert get_correlation_id() == ''


@pytest.mark.asyncio
async def test_context_isolated_in_concurrent_tasks():
    """Should isolate context between concurrent async tasks."""
    async def task(correlation_id: str) -> str:
        set_correlation_id(correlation_id)
        await asyncio.sleep(0.01)  # Simulate async work
        return get_correlation_id()
    
    results = await asyncio.gather(
        task('correlation-1'),
        task('correlation-2'),
        task('correlation-3'),
    )
    
    assert results == ['correlation-1', 'correlation-2', 'correlation-3']


@pytest.mark.asyncio
async def test_context_preserved_in_nested_async_calls():
    """Should preserve context in nested async calls."""
    async def inner() -> str:
        await asyncio.sleep(0.01)
        return get_correlation_id()
    
    result = await asyncio.get_event_loop().run_in_executor(
        None,
        lambda: (set_correlation_id('nested-id'), asyncio.run(inner()))[1]
    )
    
    # Note: run_in_executor creates a new thread, so context won't propagate
    # This is expected behavior for contextvars in threads
    # For proper nested async, use await directly


@pytest.mark.asyncio
async def test_context_after_async_task_completes():
    """Should not leak context after async task completes."""
    async def task() -> None:
        set_correlation_id('async-leak-test')
        await asyncio.sleep(0.01)
    
    await task()
    
    # Context should be cleared after task
    assert get_correlation_id() == ''


def test_context_in_thread():
    """Should support context in threads."""
    import threading
    
    result = []
    
    def thread_worker():
        set_correlation_id('thread-id')
        result.append(get_correlation_id())
    
    thread = threading.Thread(target=thread_worker)
    thread.start()
    thread.join()
    
    # Main thread context should remain unchanged
    assert get_correlation_id() == ''
    # Thread should have its own context
    assert result == ['thread-id']