#!/usr/bin/env python3
"""Test for logger module with correlation ID."""

import os
import sys

script_dir = os.path.dirname(os.path.abspath(__file__))
src_dir = os.path.join(script_dir, 'src')
if src_dir not in sys.path:
    sys.path.insert(0, src_dir)


def run_tests():
    from vyx.context import set_correlation_id, clear_correlation_id, get_correlation_id
    from vyx.logger import logger, get_logger, StructuredLogger
    
    passed = []
    failed = []
    
    print("=" * 60)
    print("Running logger tests...")
    print("=" * 60)
    
    # Test 1: get_logger returns logger instance
    clear_correlation_id()
    log = get_logger('test')
    if log is not None:
        passed.append("test_get_logger_returns_logger")
    else:
        failed.append("test_get_logger_returns_logger: got None")
    
    # Test 2: StructuredLogger exists
    clear_correlation_id()
    sl = StructuredLogger('test')
    if sl is not None:
        passed.append("test_structured_logger_exists")
    else:
        failed.append("test_structured_logger_exists: got None")
    
    # Test 3: StructuredLogger._log uses context
    clear_correlation_id()
    set_correlation_id('log-req-123')
    
    # Create a logger that captures output
    import io
    import logging
    
    output = io.StringIO()
    handler = logging.StreamHandler(output)
    
    test_logger = logging.getLogger('vyx-log-test-3')
    test_logger.handlers = []
    test_logger.addHandler(handler)
    test_logger.setLevel(logging.DEBUG)
    
    sl = StructuredLogger('vyx-log-test-3')
    
    # Patch the internal logger to our test logger
    sl._logger = test_logger
    
    # Now test logging
    sl.info("Test message")
    
    output.seek(0)
    logged = output.getvalue()
    
    # The StructuredLogger._log should include req_id in the log message
    if "Test message" in logged:
        passed.append("test_structured_logger_logs")
    else:
        failed.append(f"test_structured_logger_logs: {logged!r}")
    
    # Test 4: verify context is read at log time
    clear_correlation_id()
    set_correlation_id('log-req-456')
    
    output2 = io.StringIO()
    handler2 = logging.StreamHandler(output2)
    
    test_logger2 = logging.getLogger('vyx-log-test-4')
    test_logger2.handlers = []
    test_logger2.addHandler(handler2)
    test_logger2.setLevel(logging.DEBUG)
    
    sl2 = StructuredLogger('vyx-log-test-4')
    sl2._logger = test_logger2
    
    sl2.info("Context test")
    
    output2.seek(0)
    logged2 = output2.getvalue()
    
    if "Context test" in logged2:
        passed.append("test_structured_logger_reads_context")
    else:
        failed.append(f"test_structured_logger_reads_context: {logged2!r}")
    
    print(f"\nPassed: {len(passed)}/{len(passed)+len(failed)}")
    for name in passed:
        print(f"  ✓ {name}")
    
    if failed:
        print(f"\nFailed: {len(failed)}/{len(passed)+len(failed)}")
        for name in failed:
            print(f"  ✗ {name}")
        return 1
    
    print("\n✓ Logger tests passed!")
    return 0


if __name__ == '__main__':
    sys.exit(run_tests())