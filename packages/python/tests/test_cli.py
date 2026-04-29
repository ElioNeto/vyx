"""Tests for cli.py module."""
import json
from unittest.mock import patch, MagicMock
from io import StringIO
from pathlib import Path
import pytest

# Import after path setup
from vyx.cli import cmd_scan, main


class TestCmdScanNoRoutes:
    """Test cmd_scan with no routes."""

    def test_scan_no_routes(self, tmp_path):
        """Test scanning when no routes found."""
        # Create a file without route annotations
        test_file = tmp_path / "no_routes.py"
        test_file.write_text("x = 1\n")

        args = MagicMock()
        args.dir = None
        args.files = [str(test_file)]
        args.worker_id = "python:api"
        args.output = "-"

        # Capture stderr
        with patch("sys.stderr", new_callable=StringIO) as mock_stderr:
            result = cmd_scan(args)
        
        assert result == 1
        assert "No routes found" in mock_stderr.getvalue()


class TestCmdScanWithDir:
    """Test cmd_scan with --dir option."""

    def test_scan_with_dir(self, tmp_path):
        """Test scanning a directory with route annotations."""
        # Create a file WITH route annotations
        test_file = tmp_path / "routes.py"
        test_file.write_text('# @Route(GET /api/test)\ndef handler():\n    pass\n')

        args = MagicMock()
        args.dir = str(tmp_path)
        args.files = None
        args.worker_id = "python:api"
        args.output = "-"

        # Capture stdout
        with patch("sys.stdout", new_callable=StringIO) as mock_stdout:
            result = cmd_scan(args)
        
        assert result == 0
        output = mock_stdout.getvalue()
        # Should have printed JSON
        data = json.loads(output)
        assert "routes" in data
        assert len(data["routes"]) > 0
        assert data["routes"][0]["path"] == "/api/test"

    def test_scan_with_dir_no_routes(self, tmp_path):
        """Test scanning a directory with no routes."""
        # Create a file without routes
        test_file = tmp_path / "no_routes.py"
        test_file.write_text("x = 1\n")

        args = MagicMock()
        args.dir = str(tmp_path)
        args.files = None
        args.worker_id = "python:api"
        args.output = "-"

        with patch("sys.stderr", new_callable=StringIO) as mock_stderr:
            result = cmd_scan(args)
        
        assert result == 1
        assert "No routes found" in mock_stderr.getvalue()


class TestCmdScanWithFiles:
    """Test cmd_scan with --files option."""

    def test_scan_with_files(self, tmp_path):
        """Test scanning specific files."""
        test_file = tmp_path / "routes.py"
        test_file.write_text('# @Route(GET /api/test)\ndef handler():\n    pass\n')

        args = MagicMock()
        args.dir = None
        args.files = [str(test_file)]
        args.worker_id = "python:api"
        args.output = "-"

        with patch("sys.stdout", new_callable=StringIO) as mock_stdout:
            result = cmd_scan(args)
        
        assert result == 0
        output = mock_stdout.getvalue()
        data = json.loads(output)
        assert len(data) > 0

    def test_scan_with_multiple_files(self, tmp_path):
        """Test scanning multiple files."""
        file1 = tmp_path / "routes1.py"
        file1.write_text('# @Route(GET /api/test1)\ndef handler1():\n    pass\n')
        file2 = tmp_path / "routes2.py"
        file2.write_text('# @Route(POST /api/test2)\ndef handler2():\n    pass\n')

        args = MagicMock()
        args.dir = None
        args.files = [str(file1), str(file2)]
        args.worker_id = "python:api"
        args.output = "-"

        with patch("sys.stdout", new_callable=StringIO) as mock_stdout:
            result = cmd_scan(args)
        
        assert result == 0
        output = mock_stdout.getvalue()
        data = json.loads(output)
        assert "routes" in data
        assert len(data["routes"]) == 2


class TestCmdScanOutput:
    """Test cmd_scan output options."""

    def test_scan_output_to_file(self, tmp_path):
        """Test writing route map to file."""
        test_file = tmp_path / "routes.py"
        test_file.write_text('# @Route(GET /api/test)\ndef handler():\n    pass\n')
        output_file = tmp_path / "route_map.json"

        args = MagicMock()
        args.dir = str(tmp_path)
        args.files = None
        args.worker_id = "python:api"
        args.output = str(output_file)

        result = cmd_scan(args)
        
        assert result == 0
        assert output_file.exists()
        data = json.loads(output_file.read_text())
        assert len(data) > 0

    def test_scan_output_to_stdout(self, tmp_path):
        """Test writing route map to stdout."""
        test_file = tmp_path / "routes.py"
        test_file.write_text('# @Route(GET /api/test)\ndef handler():\n    pass\n')

        args = MagicMock()
        args.dir = str(tmp_path)
        args.files = None
        args.worker_id = "python:api"
        args.output = "-"

        with patch("sys.stdout", new_callable=StringIO) as mock_stdout:
            result = cmd_scan(args)
        
        assert result == 0
        output = mock_stdout.getvalue()
        data = json.loads(output)
        assert len(data) > 0


class TestMain:
    """Test main function."""

    def test_main_no_args(self, capsys):
        """Test main with no arguments."""
        with patch("sys.argv", ["vyx"]):
            result = main()
        
        assert result == 1
        captured = capsys.readouterr()
        assert "usage" in captured.out.lower()

    def test_main_scan_command(self, tmp_path, capsys):
        """Test main with scan command."""
        test_file = tmp_path / "routes.py"
        test_file.write_text('# @Route(GET /api/test)\ndef handler():\n    pass\n')

        with patch("sys.argv", ["vyx", "scan", "--dir", str(tmp_path), "--worker-id", "python:api"]):
            with patch("sys.stdout", new_callable=StringIO) as mock_stdout:
                result = main()
        
        assert result == 0

    def test_main_unknown_command(self, capsys):
        """Test main with unknown command."""
        with patch("sys.argv", ["vyx", "unknown"]):
            try:
                result = main()
            except SystemExit:
                result = 2  # argparse exits with 2 for invalid choice
        
        assert result in (1, 2)


class TestCmdScanEdgeCases:
    """Test edge cases for cmd_scan."""

    def test_scan_empty_dir(self, tmp_path):
        """Test scanning empty directory."""
        args = MagicMock()
        args.dir = str(tmp_path)
        args.files = None
        args.worker_id = "python:api"
        args.output = "-"

        with patch("sys.stderr", new_callable=StringIO) as mock_stderr:
            result = cmd_scan(args)
        
        assert result == 1
        assert "No routes found" in mock_stderr.getvalue()

    def test_scan_with_output_flag(self, tmp_path):
        """Test that output flag is respected."""
        test_file = tmp_path / "routes.py"
        test_file.write_text('# @Route(GET /api/test)\ndef handler():\n    pass\n')
        output_file = tmp_path / "output.json"

        args = MagicMock()
        args.dir = str(tmp_path)
        args.files = None
        args.worker_id = "python:api"
        args.output = str(output_file)

        result = cmd_scan(args)
        
        assert result == 0
        assert output_file.exists()
        # Should print message to stdout
        # (we can't easily capture print, but at least it didn't crash)
