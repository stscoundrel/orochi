#!/bin/sh
set -eu

project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

python3 - "$project_dir" <<'PYTHON'
from pathlib import Path
from zipfile import ZipFile, ZipInfo, ZIP_DEFLATED
import sys

project = Path(sys.argv[1])
archive_path = project / "dist" / "function-source.zip"
archive_path.parent.mkdir(exist_ok=True)

# An explicit allowlist keeps tests, infrastructure, and credentials out.
source_files = [project / "go.mod", project / "go.sum"]
source_files += list(project.glob("*.go"))
source_files += list((project / "internal").rglob("*.go"))
source_files = [path for path in source_files if not path.name.endswith("_test.go")]

with ZipFile(archive_path, "w", compression=ZIP_DEFLATED, compresslevel=9) as archive:
    for path in sorted(source_files):
        entry = ZipInfo(path.relative_to(project).as_posix(), date_time=(1980, 1, 1, 0, 0, 0))
        entry.create_system = 3
        entry.external_attr = 0o100644 << 16
        entry.compress_type = ZIP_DEFLATED
        archive.writestr(entry, path.read_bytes())

print(archive_path)
PYTHON
