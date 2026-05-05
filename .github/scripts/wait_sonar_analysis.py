#!/usr/bin/env python3
"""Reads SonarCloud CE task status from stdin JSON and prints the latest status."""
import sys
import json

d = json.load(sys.stdin)
tasks = d.get('queue', []) + ([d['current']] if d.get('current') else [])
latest = sorted(tasks, key=lambda t: t.get('submittedAt', ''), reverse=True)
print(latest[0]['status'] if latest else 'NONE')
