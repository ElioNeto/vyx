#!/bin/bash
# Filter coverage profile to exclude files without logic (interfaces, mocks, generated files)
# Usage: filter-coverage.sh <input_profile> <output_profile>

INPUT="$1"
OUTPUT="$2"

if [ -z "$INPUT" ] || [ -z "$OUTPUT" ]; then
    echo "Usage: $0 <input_profile> <output_profile>"
    exit 1
fi

if [ ! -f "$INPUT" ]; then
    echo "ERROR: Input file '$INPUT' not found!"
    exit 1
fi

# Create temporary file for filtering
TMPFILE=$(mktemp)

# Keep the first line (mode: set) and filter out unwanted files
head -1 "$INPUT" > "$TMPFILE"

# Exclude patterns (add more as needed):
# - Any file with "mock" in the name
# - Generated files (*.gen.go, *_generated.go)
# - Files in "examples" directory
# - Files in "testdata" directory
# - Test files (_test.go) - they shouldn't count towards coverage
tail -n +2 "$INPUT" | \
    grep -v "_mock\." | \
    grep -v "\.gen\." | \
    grep -v "_generated\." | \
    grep -v "/examples/" | \
    grep -v "/testdata/" | \
    grep -v "_test\.go" >> "$TMPFILE" || true

# Check if we have any coverage data left
LINES=$(wc -l < "$TMPFILE")
if [ "$LINES" -le 1 ]; then
    echo "WARNING: All files were filtered out! Keeping original coverage."
    cp "$INPUT" "$OUTPUT"
else
    # Replace output file
    mv "$TMPFILE" "$OUTPUT"
    echo "Filtered coverage profile: $INPUT -> $OUTPUT"
    echo "Excluded patterns: _mock, .gen, _generated, /examples/, /testdata/, _test.go"
fi
