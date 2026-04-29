import pytest
import vyx

def test_version():
    assert vyx.__version__ == "0.1.0"

def test_all_contains_expected():
    assert "get_correlation_id" in vyx.__all__
    assert "set_correlation_id" in vyx.__all__
    assert "reset_correlation_id" in vyx.__all__
    assert "clear_correlation_id" in vyx.__all__
    assert "Dispatcher" in vyx.__all__
    assert "IPCPayload" in vyx.__all__
    assert "WorkerResponse" in vyx.__all__
    assert "ipc" in vyx.__all__
    assert "scanner" in vyx.__all__

def test_validate_attribute():
    # Accessing vyx.validate should return the validate module
    validate_mod = vyx.validate
    assert validate_mod is not None
    # Accessing non-existent attribute should raise AttributeError
    with pytest.raises(AttributeError):
        _ = vyx.nonexistent

def test_get_correlation_id():
    from vyx.context import set_correlation_id, get_correlation_id, clear_correlation_id
    token = set_correlation_id("test-id")
    assert get_correlation_id() == "test-id"
    clear_correlation_id()
    assert get_correlation_id() == ''
