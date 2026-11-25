#!/usr/bin/env bash
# Copyright 2026 High Five Labs
# SPDX-License-Identifier: Apache-2.0
# Download the vendor's OpenAPI specs at the commits vendor.yaml pins.
set -euo pipefail
cd "$(dirname "$0")"
mkdir -p specs
if [ ! -s specs/snowflake.json ]; then
  tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT
  git -C "$tmp" init -q && git -C "$tmp" fetch -q --depth 1 https://github.com/snowflakedb/snowflake-rest-api-specs 990e25d97236a11826c9eed40e587c2b859e5680 && git -C "$tmp" checkout -q FETCH_HEAD -- specifications
  python3 bundle_snowflake.py "$tmp/specifications" specs/snowflake.json
fi
