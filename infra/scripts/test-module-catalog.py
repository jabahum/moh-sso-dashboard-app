#!/usr/bin/env python3
"""Regression checks for feature/package drift and catalogue classification."""
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('module_catalog', Path(__file__).with_name('check-module-catalog.py'))
catalog = importlib.util.module_from_spec(spec)
spec.loader.exec_module(catalog)


class Coverage(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        for name in ('docs/module-catalog.json', 'frontend/module-sources.json',
                     'frontend/module-locks/production/package-lock.json'):
            path = self.root / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes((ROOT / name).read_bytes())
        for feature in (ROOT / 'backend/internal/features').iterdir():
            if feature.is_dir():
                (self.root / 'backend/internal/features' / feature.name).mkdir(parents=True)
        (self.root / "backend/internal/adapters/modules").mkdir(parents=True, exist_ok=True)
        for adapter in (ROOT / 'backend/internal/adapters/modules').iterdir():
            if adapter.is_dir():
                (self.root / 'backend/internal/adapters/modules' / adapter.name).mkdir(parents=True)
        for module in (ROOT / 'backend/modules').glob('*/go.mod'):
            target = self.root / module.relative_to(ROOT)
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(module.read_bytes())
        for group in ('apps', 'packages'):
            for package in (ROOT / 'frontend' / group).glob('*/package.json'):
                target = self.root / package.relative_to(ROOT)
                target.parent.mkdir(parents=True)
                target.write_bytes(package.read_bytes())

    def update(self, mutate):
        path = self.root / 'docs/module-catalog.json'
        data = json.loads(path.read_text())
        mutate(data)
        path.write_text(json.dumps(data))

    def test_complete_inventory_and_rendering_are_stable(self):
        data, pins = catalog.load_catalog(self.root)
        expected = catalog.render(data, pins)
        data['backend'].reverse()
        data['frontend'].reverse()
        self.assertEqual(catalog.render(data, pins), expected)
        self.assertEqual(expected, (ROOT / 'docs/module-coverage.md').read_text())

    def test_new_backend_feature_requires_tracking(self):
        (self.root / 'backend/internal/features/new_feature').mkdir()
        with self.assertRaisesRegex(ValueError, 'Backend catalogue drift'):
            catalog.load_catalog(self.root)

    def test_removed_backend_feature_is_not_silently_retained(self):
        self.update(lambda data: data['backend'].pop())
        with self.assertRaisesRegex(ValueError, 'Backend catalogue drift'):
            catalog.load_catalog(self.root)

    def test_published_module_cannot_return_to_features(self):
        (self.root / 'backend/internal/features/audit').mkdir()
        with self.assertRaisesRegex(ValueError, 'Published module implementation retained'):
            catalog.load_catalog(self.root)

    def test_published_module_requires_its_adapter(self):
        data = json.loads((self.root / 'docs/module-catalog.json').read_text())
        entry = next((item for item in data['backend'] if item['integration'] == 'adapter'), None)
        if entry is None:
            self.skipTest('No adapter-integrated modules remain')
        (self.root / 'backend/internal/adapters/modules' / entry['id']).rmdir()
        with self.assertRaisesRegex(ValueError, 'Backend catalogue drift'):
            catalog.load_catalog(self.root)

    def test_unpublished_feature_cannot_masquerade_as_published_adapter(self):
        (self.root / 'backend/internal/features/auth').rename(self.root / 'backend/internal/adapters/modules/auth')
        with self.assertRaisesRegex(ValueError, 'Unpublished feature must retain'):
            catalog.load_catalog(self.root)

    def test_self_contained_module_cannot_retain_adapter(self):
        (self.root / 'backend/internal/adapters/modules/geojson').mkdir()
        with self.assertRaisesRegex(ValueError, 'Self-contained module retains'):
            catalog.load_catalog(self.root)

    def test_self_contained_module_requires_source(self):
        (self.root / 'backend/modules/geojson/go.mod').unlink()
        with self.assertRaisesRegex(ValueError, 'Self-contained module source'):
            catalog.load_catalog(self.root)

    def test_unknown_frontend_relationship_is_rejected(self):
        self.update(lambda data: data['frontend'][0].update(backendFeatures=['not_a_feature']))
        with self.assertRaisesRegex(ValueError, 'Invalid frontend package'):
            catalog.load_catalog(self.root)

    def test_publishable_package_cannot_be_omitted(self):
        self.update(lambda data: data['frontend'].pop())
        with self.assertRaisesRegex(ValueError, 'cover every publishable'):
            catalog.load_catalog(self.root)

    def test_planned_feature_cannot_claim_a_runtime_module(self):
        self.update(lambda data: data['backend'][0].update(stage='planned', moduleId='not_registered'))
        with self.assertRaisesRegex(ValueError, 'Only registry-integrated'):
            catalog.load_catalog(self.root)

    def test_worker_ownership_must_be_a_list(self):
        self.update(lambda data: data['backend'][0].update(workers='untracked worker'))
        with self.assertRaisesRegex(ValueError, 'invalid backend entry'):
            catalog.load_catalog(self.root)


if __name__ == '__main__':
    unittest.main()
