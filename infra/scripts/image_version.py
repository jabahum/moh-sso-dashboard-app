#!/usr/bin/env python3
"""Resolve a numeric release version for published production images."""
import argparse
import json
from pathlib import Path
import re

ROOT = Path(__file__).resolve().parents[2]
VERSION = re.compile(r'(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)')


def image_version(component, ref, root=ROOT):
    if component == 'backend':
        version = (root / 'backend/VERSION').read_text().strip()
    elif component == 'frontend':
        version = json.loads((root / 'frontend/apps/shell/package.json').read_text()).get('version')
    else:
        raise ValueError('Unknown image component')
    if not isinstance(version, str) or not VERSION.fullmatch(version):
        raise ValueError(f'{component} production version must be numeric MAJOR.MINOR.PATCH')
    prefix = f'refs/tags/{component}/v'
    if ref.startswith(prefix) and ref[len(prefix):] != version:
        raise ValueError(f'{component} release tag does not match prepared version {version}')
    return version


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('component', choices=('backend', 'frontend'))
    parser.add_argument('--ref', required=True)
    args = parser.parse_args()
    try:
        print(image_version(args.component, args.ref))
    except (ValueError, OSError) as error:
        parser.exit(1, str(error) + '\n')


if __name__ == '__main__':
    main()
