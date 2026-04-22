#!/usr/bin/env bash
# Downloads sentence-transformers/all-MiniLM-L6-v2 into the local model cache
# in the exact layout the "local" embedding provider expects.
#
# Usage: scripts/download-minilm.sh [dest-dir]
# Default dest: $HOME/.cache/marrow/models/minilm-l6-v2
set -euo pipefail

DEST="${1:-$HOME/.cache/marrow/models/minilm-l6-v2}"
BASE="https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main"

mkdir -p "$DEST"

echo "→ Downloading tokenizer vocab"
curl -fSL "$BASE/vocab.txt" -o "$DEST/tokenizer_vocab.txt"

echo "→ Downloading model weights (safetensors)"
curl -fSL "$BASE/model.safetensors" -o "$DEST/model.safetensors"

echo "→ Done. Model installed at: $DEST"
echo
echo "Add to your marrow config:"
echo "  [embedding]"
echo "  provider   = \"local\""
echo "  model_path = \"$DEST\""
