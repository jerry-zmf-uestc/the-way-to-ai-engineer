#!/usr/bin/env python3
"""Diagram parity and safety failures; no claim about meaning or visual layout."""
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

script = Path(__file__).resolve().parents[1] / 'scripts/verify-diagrams.py'
spec = importlib.util.spec_from_file_location('diagram_check', script)
checker = importlib.util.module_from_spec(spec)
spec.loader.exec_module(checker)
SVG = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><title>sample</title><rect width="10" height="10"/></svg>'


class DiagramChecks(unittest.TestCase):
    def test_unsafe_structure_and_references(self):
        checker.safe_svg(SVG.encode(), 'sample')
        mutations = [SVG.replace('<rect', '<script/><rect'), SVG.replace('<rect', '<foreignObject/><rect'),
                     SVG.replace('<rect', '<rect onload="alert(1)"'),
                     SVG.replace('<rect', '<use href="https://example.invalid/x"/><rect'),
                     SVG.replace('<rect', '<use href="data:text/html,unsafe"/><rect'),
                     SVG.replace('<rect', '<rect fill="url(https://example.invalid/x)"'),
                     SVG.replace('<rect', '<rect style="fill:red"'),
                     SVG.replace('<rect', '<rect xml:base="https://example.invalid/"'),
                     SVG.replace('<rect', '<other:text xmlns:other="https://example.invalid/ns"/><rect'),
                     SVG.replace('<rect', '<use href="#missing"/><rect'),
                     SVG.replace('100 100', 'nan 100'),
                     '<!DOCTYPE svg [<!ENTITY fake "x">]>' + SVG]
        for value in mutations:
            with self.subTest(value=value), self.assertRaises((ValueError, checker.ET.ParseError)):
                checker.safe_svg(value.encode(), 'sample')

    def test_registry_and_rebuild_failures(self):
        for mutation in ('valid', 'stale', 'missing-source', 'extra-source', 'duplicate-output', 'missing-alt'):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                base = root / 'public/diagrams'
                base.mkdir(parents=True)
                (root / 'content/book').mkdir(parents=True)
                source = 'from pathlib import Path\nPath(__file__).with_suffix(".svg").write_text(' + repr(SVG) + ')\n'
                (base / 'sample.py').write_text(source)
                (base / 'sample.svg').write_text(SVG)
                (base / 'sources.json').write_text(json.dumps({'sample.py': ['sample.svg']}))
                (root / 'content/book/page.md').write_text('![sample](../../public/diagrams/sample.svg)')
                (root / 'content/book/chapters.json').write_text(json.dumps({'sample': {'part': 'Part 3 · Golang Agent', 'published': True, 'path': 'content/book/page.md'}}))
                if mutation == 'stale':
                    (base / 'sample.svg').write_text(SVG.replace('sample', 'stale'))
                elif mutation == 'missing-source':
                    (base / 'sample.py').unlink()
                elif mutation == 'extra-source':
                    (base / 'extra.py').write_text(source)
                elif mutation == 'duplicate-output':
                    (base / 'sources.json').write_text(json.dumps({'sample.py': ['sample.svg', 'sample.svg']}))
                elif mutation == 'missing-alt':
                    (root / 'content/book/page.md').write_text('![](../../public/diagrams/sample.svg)')
                if mutation == 'valid':
                    checker.verify(root)
                else:
                    with self.assertRaises(ValueError):
                        checker.verify(root)


if __name__ == '__main__':
    unittest.main()
