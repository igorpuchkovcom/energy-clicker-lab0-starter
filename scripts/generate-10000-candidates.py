#!/usr/bin/env python3
import json
import sys

count = int(sys.argv[1]) if len(sys.argv) > 1 else 10_000
output = sys.argv[2] if len(sys.argv) > 2 else "/tmp/mirrors-10000.json"

candidates = [
    {
        "id": f"candidate-{index:05d}",
        "url": "http://localhost:18081",
    }
    for index in range(count)
]

with open(output, "w", encoding="utf-8") as handle:
    json.dump(candidates, handle)

print(f"Wrote {count} candidates to {output}")
