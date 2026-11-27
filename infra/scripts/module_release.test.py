#!/usr/bin/env python3
"""Offline tests for module_release.py (no network, no Go toolchain)."""
import json
from pathlib import Path
import sys
import tempfile
import unittest
from unittest import mock

sys.path.insert(0, str(Path(__file__).resolve().parent))
import module_release as mr  # noqa: E402

PREFIX = mr.MODULE_PREFIX


def write(base, files):
    for name, data in files.items():
        path = base / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(data)


class VersionTests(unittest.TestCase):
    def test_semver_ordering(self):
        self.assertLess(mr.version_key('v0.2.0'), mr.version_key('v0.3.0'))
        self.assertLess(mr.version_key('v0.9.9'), mr.version_key('v0.10.0'))

    def test_pseudo_version_sorts_before_its_next_release(self):
        pseudo = 'v0.1.1-0.20261120164120-d9bb6f8c819f'
        self.assertLess(mr.version_key('v0.1.0'), mr.version_key(pseudo))
        self.assertLess(mr.version_key(pseudo), mr.version_key('v0.1.1'))

    def test_rejects_unknown_versions(self):
        with self.assertRaises(mr.ReleaseError):
            mr.version_key('latest')


class ArtifactTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        base = Path(self.tmp.name)
        self.local, self.published, self.license = base / 'local', base / 'published', base / 'LICENSE'
        self.license.write_bytes(b'MIT License\n')

    def tearDown(self):
        self.tmp.cleanup()

    def test_identical_trees_match(self):
        files = {'go.mod': b'module x\n', 'pkg/a.go': b'package pkg\n'}
        write(self.local, files)
        write(self.published, files)
        self.assertEqual(mr.artifact_differences(self.local, self.published, self.license), [])

    def test_root_license_added_by_go_is_ignored(self):
        write(self.local, {'go.mod': b'module x\n'})
        write(self.published, {'go.mod': b'module x\n', 'LICENSE': b'MIT License\n'})
        self.assertEqual(mr.artifact_differences(self.local, self.published, self.license), [])
        write(self.local, {'LICENSE': b'MIT License\n'})
        self.assertEqual(mr.artifact_differences(self.local, self.published, self.license), [])

    def test_reports_every_kind_of_difference(self):
        write(self.local, {'go.mod': b'module x\n', 'new.go': b'package x\n', 'LICENSE': b'Other license\n'})
        write(self.published, {'go.mod': b'module y\n', 'old.go': b'package x\n'})
        differences = mr.artifact_differences(self.local, self.published, self.license)
        self.assertIn('only in source: new.go', differences)
        self.assertIn('only in source: LICENSE', differences)
        self.assertIn('only in release: old.go', differences)
        self.assertIn('changed: go.mod', differences)


class PendingTests(unittest.TestCase):
    def releases(self, data):
        handle = tempfile.NamedTemporaryFile('w', suffix='.json', delete=False)
        json.dump(data, handle)
        handle.close()
        self.addCleanup(Path(handle.name).unlink)
        return Path(handle.name)

    def test_loads_valid_entries(self):
        path = self.releases({'releases': {'audit': {'version': 'v0.3.0', 'notes': 'Accept core.Platform.'}}})
        self.assertEqual(mr.load_pending(path, known={'audit'}), {'audit': 'v0.3.0'})

    def test_rejects_invalid_entries(self):
        for releases in ({'audit': {'version': '0.3.0', 'notes': 'x'}},
                         {'audit': {'version': 'v0.3.0', 'notes': ' '}},
                         {'nope': {'version': 'v0.3.0', 'notes': 'x'}}):
            with self.assertRaises(mr.ReleaseError):
                mr.load_pending(self.releases({'releases': releases}), known={'audit'})


class CheckTests(unittest.TestCase):
    """check() decisions with downloads, pins and tags stubbed."""

    def run_check(self, pins, pending, differences, tagged=(), ids=None):
        ids = ids or sorted(pins.keys() | pending.keys())
        with mock.patch.object(mr, 'module_ids', return_value=ids), \
             mock.patch.object(mr, 'load_pending', return_value=pending), \
             mock.patch.object(mr, 'module_pins', return_value={PREFIX + k: v for k, v in pins.items()}), \
             mock.patch.object(mr, 'download', return_value=Path('.')), \
             mock.patch.object(mr, 'artifact_differences', side_effect=lambda local, *_: differences.get(local.name, [])), \
             mock.patch.object(mr, 'tag_exists', side_effect=lambda name: name in tagged):
            mr.check()

    def test_released_modules_pass(self):
        self.run_check({'audit': 'v0.2.0'}, {}, {})

    def test_changed_module_without_pending_release_fails_with_instructions(self):
        with self.assertRaisesRegex(mr.ReleaseError, 'infra/modules/releases.json'):
            self.run_check({'audit': 'v0.2.0'}, {}, {'audit': ['changed: service.go']})

    def test_changed_module_with_pending_release_passes(self):
        self.run_check({'audit': 'v0.2.0'}, {'audit': 'v0.3.0'}, {'audit': ['changed: service.go']})

    def test_pending_release_must_be_newer_and_untagged(self):
        with self.assertRaisesRegex(mr.ReleaseError, 'newer'):
            self.run_check({'audit': 'v0.2.0'}, {'audit': 'v0.2.0'}, {'audit': ['changed: a.go']})
        with self.assertRaisesRegex(mr.ReleaseError, 'already exists'):
            self.run_check({'audit': 'v0.2.0'}, {'audit': 'v0.3.0'}, {'audit': ['changed: a.go']},
                           tagged={'backend/modules/audit/v0.3.0'})

    def test_pending_release_without_changes_fails(self):
        with self.assertRaisesRegex(mr.ReleaseError, 'still matches'):
            self.run_check({'audit': 'v0.2.0'}, {'audit': 'v0.3.0'}, {})

    def test_new_module_needs_a_pending_release(self):
        with self.assertRaisesRegex(mr.ReleaseError, 'not pinned'):
            self.run_check({}, {}, {}, ids=['alerts'])
        self.run_check({}, {'alerts': 'v0.1.0'}, {})

if __name__ == '__main__':
    unittest.main(verbosity=2)
