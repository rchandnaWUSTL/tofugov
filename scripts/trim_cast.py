"""Shorten long pauses in an asciicast v3 file in place.

Waits on tofu and the model run for tens of seconds. The script's own pauses
are all under 5 seconds, so only gaps longer than that get cut.
"""
import json
import sys

MAX_GAP, TRIMMED = 5.0, 0.8

path = sys.argv[1]
header, *events = open(path).read().splitlines()
hdr = json.loads(header)
hdr.pop("idle_time_limit", None)

out, total = [json.dumps(hdr)], 0.0
for line in events:
    ev = json.loads(line)
    if ev[0] > MAX_GAP:
        ev[0] = TRIMMED
    total += ev[0]
    out.append(json.dumps(ev, ensure_ascii=False))

open(path, "w").write("\n".join(out) + "\n")
print(f"{path}: {total:.0f}s")
