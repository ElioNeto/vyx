"""
CLI entrypoint for the vyx annotation scanner.

Usage:
    python -m vyx.cli scan --dir workers/ --worker-id python:api --output route_map.json
    python -m vyx.cli scan --files workers/api.py workers/admin.py --worker-id python:api
"""

import argparse
import json
import sys
from pathlib import Path

from vyx.scanner import generate_route_map, scan_directory, scan_file


def cmd_scan(args) -> int:
    """Execute the scan subcommand."""
    entries = []

    if args.dir:
        entries.extend(scan_directory(args.dir, args.worker_id))

    if args.files:
        for filepath in args.files:
            entries.extend(scan_file(filepath, args.worker_id))

    if not entries:
        print("No routes found.", file=sys.stderr)
        return 1

    route_map = generate_route_map(entries)

    output = args.output
    if output == "-":
        print(json.dumps(route_map, indent=2))
    else:
        Path(output).write_text(json.dumps(route_map, indent=2))
        print(f"Wrote {len(entries)} routes to {output}")

    return 0


def main():
    parser = argparse.ArgumentParser(prog="vyx")
    subparsers = parser.add_subparsers(dest="command")

    scan_parser = subparsers.add_parser("scan", help="Scan Python files for route annotations")
    scan_parser.add_argument("--dir", type=str, help="Directory to scan recursively")
    scan_parser.add_argument("--files", nargs="+", type=str, help="Specific files to scan")
    scan_parser.add_argument("--worker-id", type=str, required=True, help="Worker ID to assign to routes")
    scan_parser.add_argument("--output", type=str, default="-", help="Output file path (default: stdout)")

    args = parser.parse_args()
    if args.command == "scan":
        return cmd_scan(args)

    parser.print_help()
    return 1


if __name__ == "__main__":
    sys.exit(main())
