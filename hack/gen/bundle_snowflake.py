# Copyright 2026 High Five Labs
# SPDX-License-Identifier: Apache-2.0
"""Merge snowflake-rest-api-specs/specifications/*.yaml into one OpenAPI file.

Each file's components are renamed <stem>.<name>, and refs between files are
rewritten to point inside the bundle. Usage: bundle_snowflake.py <dir> <out.json>
"""
import glob, json, os, re, sys

import yaml

src, out = sys.argv[1], sys.argv[2]
bundle = {"openapi": "3.0.0", "info": {"title": "Snowflake REST APIs (bundled)", "version": "bundle"},
          "paths": {}, "components": {}}


def fix(node, stem):
    if isinstance(node, dict):
        r = node.get("$ref")
        if isinstance(r, str):
            f, _, frag = r.partition("#")
            s = os.path.splitext(os.path.basename(f))[0] if f else stem
            m = re.match(r"/components/([^/]+)/(.+)", frag)
            if m:
                node = dict(node, **{"$ref": f"#/components/{m.group(1)}/{s}.{m.group(2)}"})
        return {k: fix(v, stem) for k, v in node.items()}
    if isinstance(node, list):
        return [fix(v, stem) for v in node]
    return node


for p in sorted(glob.glob(os.path.join(src, "*.yaml"))):
    stem = os.path.splitext(os.path.basename(p))[0]
    d = yaml.safe_load(open(p)) or {}
    for path, ops in (d.get("paths") or {}).items():
        bundle["paths"][path] = fix(ops, stem)
    for kind, items in (d.get("components") or {}).items():
        for name, v in (items or {}).items():
            bundle["components"].setdefault(kind, {})[f"{stem}.{name}"] = fix(v, stem)
json.dump(bundle, open(out, "w"), default=str)
print(len(bundle["paths"]), "paths")
