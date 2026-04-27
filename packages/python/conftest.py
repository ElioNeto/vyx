import os
import sys

# Ensure the package directory is in sys.path for pytest discovery
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))