from . import ipc, scanner
from . import validate as _validate
from .context import (
    clear_correlation_id,
    get_correlation_id,
    reset_correlation_id,
    set_correlation_id,
)
from .dispatch import (
    Dispatcher,
    IPCPayload,
    WorkerResponse,
)

__version__ = "0.1.0"

__all__ = [
    "get_correlation_id",
    "set_correlation_id",
    "reset_correlation_id",
    "clear_correlation_id",
    "Dispatcher",
    "IPCPayload",
    "WorkerResponse",
    "ipc",
    "scanner",
]


def __getattr__(name):
    if name == "validate":
        return _validate
    raise AttributeError(f"module {__name__!r} has no attribute {name!r}")
