#!/usr/bin/env python3
"""Parse coverage.xml and print a summary for the PR comment."""
import xml.etree.ElementTree as ET
import sys

try:
    tree = ET.parse('coverage.xml')
    root = tree.getroot()
    print(f'Line rate: {float(root.get("line-rate", 0))*100:.1f}%')
    for pkg in root.findall('.//package'):
        name = pkg.get('name', '')
        rate = float(pkg.get('line-rate', 0)) * 100
        print(f'  {name}: {rate:.1f}%')
except FileNotFoundError:
    print("Python coverage file not found")
