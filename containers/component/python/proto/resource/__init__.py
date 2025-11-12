# Resource proto package
# This __init__.py makes parent modules available to subdirectories
import sys
import os

# Add current directory to path so subdirectories can import parent modules
_current_dir = os.path.dirname(os.path.abspath(__file__))
if _current_dir not in sys.path:
    sys.path.insert(0, _current_dir)
