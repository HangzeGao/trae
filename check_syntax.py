#!/usr/bin/env python3
"""
Simple syntax check for SkySense++ model.
"""

import ast
import sys

def check_syntax(file_path):
    """Check Python file syntax."""
    try:
        with open(file_path, 'r', encoding='utf-8') as f:
            code = f.read()
        ast.parse(code)
        print(f"✓ {file_path}: Syntax OK")
        return True
    except SyntaxError as e:
        print(f"✗ {file_path}: Syntax Error at line {e.lineno}")
        print(f"  {e.msg}")
        return False
    except Exception as e:
        print(f"✗ {file_path}: Error - {e}")
        return False

def main():
    files_to_check = [
        '/workspace/models/skysense_pp.py',
        '/workspace/models/unet.py',
        '/workspace/models/__init__.py'
    ]
    
    print("Checking SkySense++ integration syntax...\n")
    
    all_ok = True
    for file_path in files_to_check:
        if not check_syntax(file_path):
            all_ok = False
    
    print("\n" + "=" * 60)
    if all_ok:
        print("All syntax checks passed! ✓")
    else:
        print("Some syntax checks failed! ✗")
    print("=" * 60)
    
    return 0 if all_ok else 1

if __name__ == '__main__':
    sys.exit(main())
