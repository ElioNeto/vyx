"""
Annotation scanner for Python worker files.

Parses # @Route, # @Auth, # @Validate comments above function/class definitions
and returns a list of RouteEntry objects.
"""

import re
from dataclasses import dataclass
from pathlib import Path


@dataclass
class AuthConfig:
    required: bool
    roles: list[str]


@dataclass
class ValidateConfig:
    type: str
    model: str


@dataclass
class SourceLocation:
    file: str
    line: int
    symbol: str


@dataclass
class RouteEntry:
    method: str
    path: str
    worker_id: str
    auth: AuthConfig | None
    validate: ValidateConfig | None
    source: SourceLocation


route_re = re.compile(r'@Route\((\w+)\s+([^)]+)\)')
auth_re = re.compile(r'@Auth\(roles:\s*\[([^\]]*)\]\)')
validate_re = re.compile(r'@Validate\((\w+)\)')


def _parse_auth(pending_auth: str | None) -> AuthConfig | None:
    """Parse auth annotation string into AuthConfig."""
    if not pending_auth:
        return None
    am = auth_re.match(pending_auth)
    if not am:
        return None
    roles_str = am.group(1)
    roles = [r.strip().strip('"') for r in roles_str.split(',') if r.strip()]
    return AuthConfig(required=True, roles=roles)


def _parse_validate(pending_validate: str | None, symbol: str) -> ValidateConfig | None:
    """Parse validate annotation string into ValidateConfig."""
    if not pending_validate:
        return None
    vm = validate_re.match(pending_validate)
    if not vm:
        return None
    validate_type = vm.group(1).strip()
    return ValidateConfig(type=validate_type, model=symbol)


def _is_annotation(line: str) -> str | None:
    """Check if line is an annotation comment. Returns the annotation text or None."""
    if not line.startswith('#'):
        return None
    text = line[1:].strip()
    if route_re.match(text) or auth_re.match(text) or validate_re.match(text):
        return text
    return None


def _is_symbol_definition(line: str) -> str | None:
    """Check if line defines a function or class. Returns the symbol name or None."""
    if line.startswith('def ') or line.startswith('class ') or line.startswith('async def '):
        match = re.match(r'(?:async\s+)?(?:def|class)\s+(\w+)', line)
        if match:
            return match.group(1)
    return None


def scan_file(filepath: str | Path, worker_id: str) -> list[RouteEntry]:
    """
    Scan a single Python file for @Route annotations.
    Returns a list of RouteEntry for every annotated function/class found.
    """
    path = Path(filepath)
    if not path.exists():
        return []

    lines = path.read_text().splitlines()
    entries = []

    pending_route = None
    pending_auth = None
    pending_validate = None

    for i, line in enumerate(lines):
        stripped = line.strip()

        if annotation := _is_annotation(stripped):
            pending_route, pending_auth, pending_validate = _update_pending(
                annotation, pending_route, pending_auth, pending_validate)
            continue

        if symbol := _is_symbol_definition(stripped):
            entry = _maybe_build_entry(
                path, i, symbol, worker_id,
                pending_route, pending_auth, pending_validate)
            if entry:
                entries.append(entry)
            pending_route = None
            pending_auth = None
            pending_validate = None
            continue

        if stripped == '':
            pending_route = None
            pending_auth = None
            pending_validate = None

    return entries


def _update_pending(annotation: str, pr: str | None, pa: str | None, pv: str | None) -> tuple[str | None, str | None, str | None]:
    """Update pending annotations based on annotation type."""
    if route_re.match(annotation):
        return annotation, pa, pv
    if auth_re.match(annotation):
        return pr, annotation, pv
    if validate_re.match(annotation):
        return pr, pa, annotation
    return pr, pa, pv


def _maybe_build_entry(
    path: Path,
    line_idx: int,
    symbol: str,
    worker_id: str,
    pending_route: str | None,
    pending_auth: str | None,
    pending_validate: str | None,
) -> RouteEntry | None:
    """Build a RouteEntry if there's a pending route annotation."""
    if not pending_route:
        return None

    m = route_re.match(pending_route)
    if not m:
        return None

    return RouteEntry(
        method=m.group(1).upper(),
        path=m.group(2).strip(),
        worker_id=worker_id,
        auth=_parse_auth(pending_auth),
        validate=_parse_validate(pending_validate, symbol),
        source=SourceLocation(file=str(path), line=line_idx + 1, symbol=symbol),
    )


def scan_directory(directory: str | Path, worker_id: str, pattern: str = "**/*.py") -> list[RouteEntry]:
    """
    Recursively scan all Python files in a directory.
    """
    dir_path = Path(directory)
    if not dir_path.exists() or not dir_path.is_dir():
        return []

    entries = []
    for filepath in dir_path.glob(pattern):
        if filepath.is_file():
            entries.extend(scan_file(filepath, worker_id))

    return entries


def generate_route_map(entries: list[RouteEntry]) -> dict:
    """
    Serialize RouteEntry list to the route_map.json dict structure.
    """
    routes = []

    for entry in entries:
        route = {
            "method": entry.method,
            "path": entry.path,
            "worker_id": entry.worker_id,
            "auth": None,
            "validate": None,
            "source": {
                "file": entry.source.file,
                "line": entry.source.line,
                "symbol": entry.source.symbol
            }
        }

        if entry.auth:
            route["auth"] = {
                "required": entry.auth.required,
                "roles": entry.auth.roles
            }

        if entry.validate:
            route["validate"] = {
                "type": entry.validate.type,
                "model": entry.validate.model
            }

        routes.append(route)

    return {"routes": routes}
