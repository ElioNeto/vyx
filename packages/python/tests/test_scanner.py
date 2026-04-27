"""Tests for the Python annotation scanner."""

import json
import textwrap
import pytest
from pathlib import Path
from vyx.scanner import scan_file, scan_directory, generate_route_map


SIMPLE_ROUTE = textwrap.dedent("""\
    # @Route(GET /api/users)
    def list_users(req):
        return []
""")

ROUTE_WITH_AUTH = textwrap.dedent("""\
    # @Route(POST /api/orders)
    # @Auth(roles: ["user", "admin"])
    def create_order(req):
        return {}
""")

ROUTE_WITH_VALIDATE = textwrap.dedent("""\
    # @Route(POST /api/items)
    # @Validate(pydantic)
    class ItemInput:
        name: str
""")

ASYNC_ROUTE = textwrap.dedent("""\
    # @Route(GET /api/users/:id)
    async def get_user(req):
        return {}
""")

NO_ANNOTATION = textwrap.dedent("""\
    def helper():
        pass
""")

BROKEN_ANNOTATION = textwrap.dedent("""\
    # @Route(GET /api/broken)

    def detached_function(req):
        pass
""")


def test_scan_simple_route(tmp_path):
    f = tmp_path / "api.py"
    f.write_text(SIMPLE_ROUTE)
    routes = scan_file(f, "python:api")
    assert len(routes) == 1
    assert routes[0].method == "GET"
    assert routes[0].path == "/api/users"
    assert routes[0].worker_id == "python:api"
    assert routes[0].auth is None
    assert routes[0].validate is None


def test_scan_route_with_auth(tmp_path):
    f = tmp_path / "api.py"
    f.write_text(ROUTE_WITH_AUTH)
    routes = scan_file(f, "python:api")
    assert len(routes) == 1
    assert routes[0].auth.required is True
    assert routes[0].auth.roles == ["user", "admin"]


def test_scan_route_with_validate(tmp_path):
    f = tmp_path / "api.py"
    f.write_text(ROUTE_WITH_VALIDATE)
    routes = scan_file(f, "python:api")
    assert len(routes) == 1
    assert routes[0].validate.type == "pydantic"
    assert routes[0].validate.model == "ItemInput"


def test_scan_async_route(tmp_path):
    f = tmp_path / "api.py"
    f.write_text(ASYNC_ROUTE)
    routes = scan_file(f, "python:api")
    assert len(routes) == 1
    assert routes[0].source.symbol == "get_user"


def test_scan_ignores_unannotated_functions(tmp_path):
    f = tmp_path / "api.py"
    f.write_text(NO_ANNOTATION)
    routes = scan_file(f, "python:api")
    assert len(routes) == 0


def test_scan_ignores_detached_annotation(tmp_path):
    """Empty line between comment and def must cancel association."""
    f = tmp_path / "api.py"
    f.write_text(BROKEN_ANNOTATION)
    routes = scan_file(f, "python:api")
    assert len(routes) == 0


def test_scan_records_source_location(tmp_path):
    f = tmp_path / "api.py"
    f.write_text(SIMPLE_ROUTE)
    routes = scan_file(f, "python:api")
    assert routes[0].source.line == 2
    assert routes[0].source.symbol == "list_users"
    assert "api.py" in routes[0].source.file


def test_scan_directory(tmp_path):
    (tmp_path / "api.py").write_text(SIMPLE_ROUTE)
    (tmp_path / "admin.py").write_text(ROUTE_WITH_AUTH)
    routes = scan_directory(tmp_path, "python:api")
    assert len(routes) == 2


def test_generate_route_map_schema(tmp_path):
    f = tmp_path / "api.py"
    f.write_text(ROUTE_WITH_AUTH)
    routes = scan_file(f, "python:api")
    route_map = generate_route_map(routes)
    assert "routes" in route_map
    r = route_map["routes"][0]
    assert r["method"] == "POST"
    assert r["path"] == "/api/orders"
    assert r["worker_id"] == "python:api"
    assert r["auth"]["required"] is True
    assert r["auth"]["roles"] == ["user", "admin"]
    assert r["validate"] is None