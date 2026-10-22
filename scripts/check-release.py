#!/usr/bin/env python3
"""Check checksums and exact archive contents after build-release.sh."""
import hashlib
from pathlib import Path
import sys
import tarfile
import zipfile

folder = Path(sys.argv[1] if len(sys.argv) > 1 else "dist")
lines = (folder / "checksums.txt").read_text().splitlines()
assert len(lines) == 6, "expected six platform archives"
for line in lines:
    digest, name = line.split()
    path = folder / name
    assert hashlib.sha256(path.read_bytes()).hexdigest() == digest, f"checksum mismatch: {name}"
    if path.suffix == ".zip":
        with zipfile.ZipFile(path) as archive:
            names = archive.namelist()
        expected = ["LICENSE", "README.md", "reqdiff.exe"]
    else:
        with tarfile.open(path) as archive:
            names = archive.getnames()
        expected = ["LICENSE", "README.md", "reqdiff"]
    assert sorted(names) == expected, f"unexpected archive entries in {name}: {names}"
print("Verified six release archives and SHA-256 checksums")
