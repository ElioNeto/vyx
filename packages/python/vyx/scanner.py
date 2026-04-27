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

        if stripped.startswith('#'):
            comment = stripped[1:].strip()

            if route_re.match(comment):
                pending_route = comment
            elif auth_re.match(comment):
                pending_auth = comment
            elif validate_re.match(comment):
                pending_validate = comment
            continue

        if stripped.startswith('def ') or stripped.startswith('class ') or stripped.startswith('async def '):
            symbol_match = re.match(r'(?:async\s+)?(?:def|class)\s+(\w+)', stripped)
            if not symbol_match:
                continue

            symbol = symbol_match.group(1)
            line_num = i + 1

            if pending_route:
                m = route_re.match(pending_route)
                method = m.group(1).upper()
                route_path = m.group(2).strip()

                auth_cfg = None
                if pending_auth:
                    am = auth_re.match(pending_auth)
                    if am:
                        roles_str = am.group(1)
                        roles = [r.strip().strip('"') for r in roles_str.split(',') if r.strip()]
                        auth_cfg = AuthConfig(required=True, roles=roles)

                validate_cfg = None
                if pending_validate:
                    vm = validate_re.match(pending_validate)
                    if vm:
                        validate_type = vm.group(1).strip()
                        validate_cfg = ValidateConfig(type=validate_type, model=symbol)

                source = SourceLocation(
                    file=str(path),
                    line=line_num,
                    symbol=symbol
                )

                entry = RouteEntry(
                    method=method,
                    path=route_path,
                    worker_id=worker_id,
                    auth=auth_cfg,
                    validate=validate_cfg,
                    source=source
                )
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
