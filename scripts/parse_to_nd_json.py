#!/usr/bin/env python3
"""
Parse NCC ARL single-choice exam text to NDJSON.

Input format (exams/ncc-arl-exam-1.txt):
  #section <section name>
  #correct=<answer>, num=<num> <question start>
  <question continuation lines...>
  (1) <option 1>
  <option 1 continuation...>
  (2) <option 2>
  ...
  (4) <option 4>

- Question and option texts may wrap across multiple lines.
- Options use half-width "(1)" or full-width "（1）" markers.
- One malformed entry (num=22, 170Hz/300baud) has options BEFORE question text;
  the parser handles that by detecting when the #correct line itself starts with an option marker.

Output NDJSON per line:
  {
    "id": "<counter>",
    "section": "<section>",
    "answer": ["<correct>"],
    "question": "<question>",
    "options": [
      {"option_id": "1", "option": "..."},
      ...
    ]
  }

Usage:
  python scripts/parse_to_nd_json.py exams/ncc-arl-exam-1.txt -o output.ndjson
  python scripts/parse_to_nd_json.py exams/ncc-arl-exam-1.txt > output.ndjson
  python scripts/parse_to_nd_json.py --help
"""

import argparse
import json
import re
import sys
from pathlib import Path

# Matches "(1) text" or "（1） text" with optional leading spaces
OPTION_RE = re.compile(r"^\s*[\(（]\s*(\d+)\s*[\)）]\s*(.*)$")
# Matches "#correct=4, num=1 <question start>"
HEADER_RE = re.compile(r"^#correct\s*=\s*(\d+)\s*,\s*num\s*=\s*(\d+)\s*(.*)$")
SECTION_PREFIX = "#section"


def parse_file(path: Path):
    text = path.read_text(encoding="utf-8")
    lines = text.splitlines()

    section = ""
    counter = 0
    i = 0
    n = len(lines)

    while i < n:
        raw = lines[i]

        # Skip empty lines
        if not raw.strip():
            i += 1
            continue

        # Section header
        if raw.startswith(SECTION_PREFIX):
            section = raw[len(SECTION_PREFIX) :].strip()
            i += 1
            continue

        # Question header
        if raw.startswith("#correct"):
            m = HEADER_RE.match(raw)
            if not m:
                # Unrecognized header, skip
                sys.stderr.write(
                    f"warn: unrecognized header at line {i + 1}: {raw!r}\n"
                )
                i += 1
                continue

            correct, num, qstart = m.groups()
            correct = correct.strip()
            qstart = qstart.strip()
            counter += 1

            # Detect malformed case where qstart itself is an option like "(1) 0.1 赫"
            # In that case options come first, question text after.
            if OPTION_RE.match(qstart):
                # Collect 4 options starting with this line — no continuations,
                # because the trailing question text would otherwise be consumed
                # as continuation of option 4.
                opts = []
                om = OPTION_RE.match(qstart)
                assert om is not None
                oid, otext = om.groups()
                opts.append((oid.strip(), otext.strip()))
                i += 1
                while i < n and len(opts) < 4:
                    if lines[i].startswith("#correct") or lines[i].startswith(
                        SECTION_PREFIX
                    ):
                        break
                    om2 = OPTION_RE.match(lines[i])
                    if om2:
                        oid2, otext2 = om2.groups()
                        opts.append((oid2.strip(), otext2.strip()))
                        i += 1
                    else:
                        # Skip stray/blank lines between options
                        i += 1
                # Question text is the lines after the 4 options until next header/section
                q_parts = []
                while (
                    i < n
                    and not lines[i].startswith("#correct")
                    and not lines[i].startswith(SECTION_PREFIX)
                ):
                    if lines[i].strip():
                        if OPTION_RE.match(lines[i]):
                            break
                        q_parts.append(lines[i].strip())
                    i += 1
                question = "".join(q_parts)

                # If question is still empty, fallback to empty string (should not happen)
                # Sort options by numeric id to ensure order
                opts_sorted = sorted(opts, key=lambda x: int(x[0]))
                yield {
                    "id": str(counter),
                    "section": section,
                    "answer": [correct],
                    "question": question,
                    "options": [
                        {"option_id": oid, "option": opt} for oid, opt in opts_sorted
                    ],
                }
                continue

            # Normal case: question text before options
            q_parts = [qstart] if qstart else []
            i += 1

            # Collect question continuation lines until first option or next header
            while (
                i < n
                and not OPTION_RE.match(lines[i])
                and not lines[i].startswith("#correct")
                and not lines[i].startswith(SECTION_PREFIX)
            ):
                if lines[i].strip():
                    q_parts.append(lines[i].strip())
                i += 1
            question = "".join(q_parts)

            # Collect 4 options
            opts = []
            while i < n and len(opts) < 4:
                if lines[i].startswith("#correct") or lines[i].startswith(
                    SECTION_PREFIX
                ):
                    break
                om = OPTION_RE.match(lines[i])
                if om:
                    oid, otext = om.groups()
                    otext = otext.strip()
                    i += 1
                    # Collect continuation lines for this option
                    cont = []
                    while (
                        i < n
                        and not OPTION_RE.match(lines[i])
                        and not lines[i].startswith("#correct")
                        and not lines[i].startswith(SECTION_PREFIX)
                    ):
                        if lines[i].strip():
                            cont.append(lines[i].strip())
                        else:
                            i += 1
                            continue
                        i += 1
                    full = otext + "".join(cont)
                    opts.append((oid.strip(), full))
                else:
                    # Unexpected non-option line inside options block (should be continuation already handled)
                    # Skip blank or stray
                    if lines[i].strip():
                        sys.stderr.write(
                            f"warn: stray line inside options at {i + 1}: {lines[i]!r} (q {counter})\n"
                        )
                    i += 1

            if len(opts) != 4:
                sys.stderr.write(
                    f"warn: question {counter} (num={num}, section={section}) has {len(opts)} options, expected 4. question={question[:60]!r}\n"
                )

            # Sort by option_id numeric to ensure stable order
            opts_sorted = sorted(
                opts, key=lambda x: int(x[0]) if x[0].isdigit() else x[0]
            )

            yield {
                "id": str(counter),
                "section": section,
                "answer": [correct],
                "question": question,
                "options": [
                    {"option_id": oid, "option": opt} for oid, opt in opts_sorted
                ],
            }
            continue

        # Anything else (should not happen outside question blocks)
        i += 1


def main():
    parser = argparse.ArgumentParser(
        description="Parse NCC ARL single-choice exam text to NDJSON"
    )
    parser.add_argument(
        "input",
        nargs="?",
        default="exams/ncc-arl-exam-1.txt",
        help="Input text file (default: exams/ncc-arl-exam-1.txt)",
    )
    parser.add_argument(
        "-o", "--output", default=None, help="Output NDJSON file (default: stdout)"
    )
    parser.add_argument(
        "--ensure-ascii",
        action="store_true",
        help="Escape non-ASCII (default: preserve Unicode)",
    )
    args = parser.parse_args()

    input_path = Path(args.input)
    if not input_path.exists():
        sys.stderr.write(f"error: input file not found: {input_path}\n")
        sys.exit(1)

    out_f = open(args.output, "w", encoding="utf-8") if args.output else sys.stdout

    try:
        count = 0
        for record in parse_file(input_path):
            json.dump(
                record, out_f, ensure_ascii=args.ensure_ascii, separators=(",", ":")
            )
            out_f.write("\n")
            count += 1
        if args.output:
            sys.stderr.write(f"wrote {count} questions to {args.output}\n")
        else:
            sys.stderr.write(f"wrote {count} questions to stdout\n")
    finally:
        if args.output:
            out_f.close()


if __name__ == "__main__":
    main()
