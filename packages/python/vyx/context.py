from contextvars import ContextVar, Token

_correlation_id: ContextVar[str] = ContextVar('correlation_id', default='')


def get_correlation_id() -> str:
    """Get the correlation ID for the current request context.

    Returns empty string if not in a request context.
    """
    return _correlation_id.get()


def set_correlation_id(value: str) -> Token:
    """Set the correlation ID for the current request context.

    Returns a Token that can be used with reset_correlation_id to restore
    the previous value.
    """
    return _correlation_id.set(value)


def reset_correlation_id(token: Token) -> None:
    """Reset correlation ID to its previous value using the token from set_correlation_id."""
    _correlation_id.reset(token)


def clear_correlation_id() -> None:
    """Clear the correlation ID (set to empty string).

    Use reset_correlation_id(token) when you have a token from set_correlation_id
    and want to restore the exact previous value.
    """
    _correlation_id.set('')
