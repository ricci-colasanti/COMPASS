#!/usr/bin/env python3
# -------------------------------------------------
# Python script: test.py
# -------------------------------------------------

import json
import subprocess
import sys
from pathlib import Path

# ----------------------------------------------------------------------
# 1️  Build the payload (mirrors the Go structs)
# ----------------------------------------------------------------------
payload = {
    "constraints" : "data/BlockWorld/artifical_cencus.csv",
    "microdata"   : "data/BlockWorld/artifical_survay.csv",
    "groups"      : "data/BlockWorld/artificial_groups.csv",
    "output"      : "results/artificial_synthetic_population.csv",
    "validate"    : "results/artificial_synthPopSurvey.csv",
    "initialTemp":      1000.0,
    "minTemp":          0.001,
    "coolingRate":      0.997,
    "reheatFactor":     0.3,
    "fitnessThreshold": 0.01,
    "minImprovement":   0.001,
    "maxIterations":    500000,
    "windowSize":       5000,
    "change":           100000,
    "distance":         "NORM_EUCLIDEAN",
    "useRandomSeed":    "no",
    "randomSeed":       42
}

# ----------------------------------------------------------------------
# 2️  Serialise to **compact** JSON (no spaces, no pretty‑print)
# ----------------------------------------------------------------------
json_in = json.dumps(payload, separators=(',', ':'), ensure_ascii=False)

# ----------------------------------------------------------------------
# 3️  Locate the compiled Go binary
# ----------------------------------------------------------------------
# By default we assume it lives next to this script (./compass)
binary = "./COMPASS"





# ----------------------------------------------------------------------
# 4️  Run the binary, feeding the JSON via stdin
# ----------------------------------------------------------------------
try:
    completed = subprocess.run(
        [binary],   # command line (list of args)
        input=json_in,        # JSON payload → child's stdin
        capture_output=True,  # grab both stdout and stderr
        text=True,           # work with strings (not bytes)
        check=False          # we'll handle non‑zero exit codes ourselves
    )
except OSError as exc:
    sys.stderr.write(f" Failed to launch the binary: {exc}\n")
    sys.exit(1)

# ----------------------------------------------------------------------
# 5️  Check the exit status (non‑zero means the Go program hit an error)
# ----------------------------------------------------------------------
if completed.returncode != 0:
    sys.stderr.write(
        f" Binary exited with code {completed.returncode}\n"
        f"--- STDOUT (may contain JSON error) ---\n{completed.stdout}\n"
        f"--- STDERR (diagnostic prints) ---\n{completed.stderr}\n"
    )
    # We still try to parse whatever JSON we got (it may be an error object).

# ----------------------------------------------------------------------
# 6️  Parse the JSON that the binary emitted on stdout
# ----------------------------------------------------------------------
try:
    result = json.loads(completed.stdout)
except json.JSONDecodeError as exc:
    sys.stderr.write(
        f" Failed to decode JSON from the binary's stdout:\n"
        f"{completed.stdout}\n"
        f"Error: {exc}\n"
    )
    sys.exit(1)

# ----------------------------------------------------------------------
# 7️  Display the result (mirrors what you printed in R)
# ----------------------------------------------------------------------
print("=== Binary response ===")
print(json.dumps(result, indent=2, ensure_ascii=False))

# If you want to work with the fields directly:
status  = result.get("status")
message = result.get("message")
log     = result.get("log", [])   # only present when called from Python (R‑mode)

print("\nStatus :", status)
print("Message:", message)

if log:
    print("\n--- Log captured from the Go binary ---")
    for line in log:
        print(line)