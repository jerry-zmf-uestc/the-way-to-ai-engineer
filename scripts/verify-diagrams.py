#!/usr/bin/env python3
"""Check registered Python/SVG artifacts, not diagram teaching semantics."""
import json
import math
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
import xml.etree.ElementTree as ET


def safe_svg(data, name):
    if len(data) > 1 << 20 or b'<!DOCTYPE' in data.upper() or b'<!ENTITY' in data.upper():
        raise ValueError(f'{name}: SVG declaration/size')
    tree = ET.fromstring(data)
    if tree.tag != '{http://www.w3.org/2000/svg}svg':
        raise ValueError(f'{name}: SVG root')
    box = [float(x) for x in tree.attrib.get('viewBox', '').split()]
    if len(box) != 4 or not all(math.isfinite(x) for x in box) or box[2] <= 0 or box[3] <= 0:
        raise ValueError(f'{name}: viewBox')
    tags = {'svg', 'title', 'desc', 'defs', 'marker', 'g', 'rect', 'text', 'tspan', 'path', 'line', 'circle', 'ellipse', 'polygon', 'polyline', 'use'}
    attributes = {'aria-labelledby', 'd', 'fill', 'font-family', 'font-size', 'height', 'id', 'marker-end', 'markerHeight', 'markerWidth', 'orient', 'refX', 'refY', 'role', 'rx', 'stroke', 'stroke-dasharray', 'stroke-width', 'text-anchor', 'viewBox', 'width', 'x', 'y', 'href', '{http://www.w3.org/1999/xlink}href'}
    ids = set()
    references = []
    title = False
    for node in tree.iter():
        tag = node.tag.rsplit('}', 1)[-1]
        if not node.tag.startswith('{http://www.w3.org/2000/svg}') or tag not in tags:
            raise ValueError(f'{name}: active/unknown element {tag}')
        title |= tag == 'title' and bool(''.join(node.itertext()).strip())
        for key, value in node.attrib.items():
            if key not in attributes:
                raise ValueError(f'{name}: unknown attribute {key}')
            attr = key.rsplit('}', 1)[-1].lower()
            if attr.startswith('on') or attr == 'style':
                raise ValueError(f'{name}: active attribute {attr}')
            if attr == 'id':
                if not value or value in ids:
                    raise ValueError(f'{name}: duplicate/empty id')
                ids.add(value)
            if attr in ('href', 'src'):
                if not value.startswith('#'):
                    raise ValueError(f'{name}: external reference')
                references.append(value[1:])
            for target in re.findall(r'url\((.*?)\)', value, re.I):
                target = target.strip(' \"\'')
                if not target.startswith('#'):
                    raise ValueError(f'{name}: external URL')
                references.append(target[1:])
        if 'aria-labelledby' in node.attrib:
            references.extend(node.attrib['aria-labelledby'].split())
    if not title or any(target not in ids for target in references):
        raise ValueError(f'{name}: title/local reference missing')


def verify(root):
    base = root / 'public/diagrams'
    sources = json.loads((base / 'sources.json').read_text())
    expected = set()
    for name, outputs in sources.items():
        if not re.fullmatch(r'[a-z0-9-]+\.py', name) or not isinstance(outputs, list) or not outputs:
            raise ValueError('invalid diagram source registration')
        for output in outputs:
            if not isinstance(output, str) or not re.fullmatch(r'[a-z0-9-]+\.svg', output) or output in expected:
                raise ValueError('invalid/duplicate SVG registration')
            expected.add(output)
    if {p.name for p in base.glob('*.py')} != set(sources) or {p.name for p in base.glob('*.svg')} != expected:
        raise ValueError('diagram source/artifact set differs from registry')
    for name, outputs in sources.items():
        with tempfile.TemporaryDirectory(prefix='diagram-check-') as directory:
            stage = Path(directory)
            shutil.copyfile(base / name, stage / name)
            subprocess.run([sys.executable, str(stage / name)], cwd=stage, check=True, timeout=10, capture_output=True)
            if {p.name for p in stage.glob('*.svg')} != set(outputs):
                raise ValueError(f'{name}: unexpected/missing output')
            for output in outputs:
                actual = (base / output).read_bytes()
                safe_svg(actual, output)
                if actual != (stage / output).read_bytes():
                    raise ValueError(f'{output}: source/production artifact differ')
    referenced = set()
    for entry in json.loads((root / 'book/chapters.json').read_text()).values():
        if entry['part'] != 'Part 3 · Golang Agent' or not entry['published']:
            continue
        page = root / entry['path']
        for alt, link in re.findall(r'!\[([^\]]*)\]\(([^)]+\.svg)\)', page.read_text()):
            asset = (page.parent / link).resolve()
            if not alt.strip() or asset.parent != base.resolve() or asset.name not in expected:
                raise ValueError(f'{entry["path"]}: invalid diagram reference/alt')
            referenced.add(asset.name)
    print(f'diagrams: {len(sources)} sources / {len(expected)} SVGs / {len(referenced)} referenced; parity and static structure checked, semantics separate')


if __name__ == '__main__':
    try:
        verify(Path(sys.argv[1]).resolve() if len(sys.argv) > 1 else Path(__file__).resolve().parents[1])
    except (ValueError, OSError, ET.ParseError, subprocess.SubprocessError) as error:
        sys.exit(str(error))
