#!/usr/bin/env bash
set -euo pipefail

CURRENT_HASH=$(grep -oP '(?<=vendorHash = ")[^"]+' flake.nix)

go mod vendor

COMPUTED_HASH=$(nix hash path vendor/)

rm -rf vendor/

if [ "$CURRENT_HASH" = "$COMPUTED_HASH" ]; then
  echo "vendorHash is up to date"
  exit 0
fi

echo "Updating vendorHash in flake.nix..."
python3 -c "
import sys
with open('flake.nix', 'r') as f:
    content = f.read()
content = content.replace('$CURRENT_HASH', '$COMPUTED_HASH')
with open('flake.nix', 'w') as f:
    f.write(content)
"
echo "  Old: $CURRENT_HASH"
echo "  New: $COMPUTED_HASH"
