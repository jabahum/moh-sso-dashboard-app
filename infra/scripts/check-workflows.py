#!/usr/bin/env python3
"""Lint active workflows and deployment shell scripts; tools are installed by CI."""
from pathlib import Path
import shutil
import subprocess

ROOT = Path(__file__).resolve().parents[2]


def main():
    for command in ('actionlint', 'shellcheck'):
        if not shutil.which(command):
            raise SystemExit(f'{command} is required; see docs/ci-checks.md')
    workflows = sorted(path for path in (ROOT / '.github/workflows').glob('*')
                       if path.suffix in ('.yml', '.yaml') and any(line.strip() and not line.lstrip().startswith('#') for line in path.read_text().splitlines()))
    subprocess.run(['actionlint', '-pyflakes=', *map(str, workflows)], cwd=ROOT, check=True)
    scripts = sorted([*(ROOT / 'infra').rglob('*.sh'), *(ROOT / 'infra/server/hooks').glob('*.sh.example')])
    subprocess.run(['shellcheck', '--severity=warning', *map(str, scripts)], cwd=ROOT, check=True)


if __name__ == '__main__':
    main()
