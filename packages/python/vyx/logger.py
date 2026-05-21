"""Structured JSON logger for Python workers.

Emits logs in the same format as the Node.js worker SDK so that all
vyx processes produce a unified, machine-readable log stream.

Usage:

    from vyx.logger import logger

    logger.info("request started", method="GET", path="/api/users")
    logger.error("worker crashed", exc=err)
"""

from __future__ import annotations

import json
import sys
from datetime import datetime, timezone
from typing import Any

from .context import get_correlation_id

_LOG_LEVELS = {
    "debug": 10,
    "info": 20,
    "warn": 30,
    "error": 40,
}

_LOG_LEVEL_NAMES = {v: k for k, v in _LOG_LEVELS.items()}


def _format_log(level: str, message: str, data: dict[str, Any] | None = None) -> str:
    """Format a structured log entry as JSON."""
    entry: dict[str, Any] = {
        "level": level,
        "message": message,
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "source": "PYTHON",
    }

    req_id = get_correlation_id()
    if req_id:
        entry["req_id"] = req_id

    if data:
        entry.update(data)

    return json.dumps(entry, default=str, ensure_ascii=False)


class Logger:
    """Structured JSON logger that reads correlation ID from context."""

    def __init__(self, level: str = "info") -> None:
        self._level = _LOG_LEVELS.get(level, _LOG_LEVELS["info"])

    def _log(self, level: str, message: str, **data: Any) -> None:
        if _LOG_LEVELS.get(level, 0) < self._level:
            return

        formatted = _format_log(level, message, data if data else None)

        if level == "error":
            print(formatted, file=sys.stderr)
        elif level == "warn":
            print(formatted, file=sys.stderr)
        else:
            print(formatted, file=sys.stdout)

    def debug(self, message: str, **data: Any) -> None:
        """Log at DEBUG level."""
        self._log("debug", message, **data)

    def info(self, message: str, **data: Any) -> None:
        """Log at INFO level."""
        self._log("info", message, **data)

    def warn(self, message: str, **data: Any) -> None:
        """Log at WARN level."""
        self._log("warn", message, **data)

    def error(self, message: str, **data: Any) -> None:
        """Log at ERROR level."""
        self._log("error", message, **data)


# Module-level singleton for convenience.
# Usage: from vyx.logger import logger
logger = Logger()
