#!/usr/bin/env python3
"""Create local lab credentials once. Never print their contents into evidence."""
import json
import os
from pathlib import Path
import secrets

root = Path(__file__).resolve().parents[1] / '.secrets'
root.mkdir(mode=0o700, exist_ok=True)
files = {
    'gateway-token': lambda: secrets.token_hex(32) + '\n',
    'test-users.json': lambda: json.dumps({role + '.test': secrets.token_urlsafe(24)
                                         for role in ('student', 'teacher', 'admin')}, indent=2) + '\n',
}
for name, create in files.items():
    path = root / name
    if path.exists():
        print(f'Kept .secrets/{name}')
        continue
    with os.fdopen(os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600), 'w') as stream:
        stream.write(create())
    print(f'Created .secrets/{name} (ignored by Git)')
