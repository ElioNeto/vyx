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


class TestMain:
    """Test main function."""

    def test_main_no_args(self, capsys):
        """Test main with no arguments."""
        with patch("sys.argv", ["vyx"]):
            result = main()
        
        assert result == 1
        captured = capsys.readouterr()
        assert "usage" in captured.out.lower()
