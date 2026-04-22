#!/usr/bin/env python3
"""Generate reference embeddings for the Go parity test.

Usage:
    pip install sentence-transformers
    scripts/gen-parity-fixture.py > internal/embed/local/testdata/parity.json

The resulting JSON is an array of {text, embedding} objects. The Go parity
test asserts that our pure-Go encoder produces vectors within 1e-4 cosine
distance of each reference embedding.
"""

import json
import sys

from sentence_transformers import SentenceTransformer

CANARIES = [
    "",
    "hello world",
    "The quick brown fox jumps over the lazy dog.",
    "Marrow is a local-first hybrid search engine for Markdown.",
    "sqlite-vec integrates vector search directly into SQLite.",
    "Resumé written with accents: café, naïve, façade.",
    "Punctuation: don't, can't, won't — it's fine!",
    "Numbers and symbols: 42, 3.14, $100, 50%.",
    "Multi-sentence paragraph. It has several clauses. Each matters.",
    "CJK mixed: Hello 世界, this is a test.",
    "A longer passage intended to exercise positional embeddings and the "
    "attention mechanism across a wider span of tokens than a short query "
    "would touch. This should still mean-pool cleanly.",
    "code snippet: func Add(a, b int) int { return a + b }",
    "URL: https://example.com/path?query=1&other=2",
    "Short.",
    "A",
    "search query about machine learning papers",
    "implementation details of self-attention in transformers",
    "what is the capital of France",
    "Paris is the capital of France.",
    "London is the capital of the United Kingdom.",
]

def main() -> None:
    model = SentenceTransformer("sentence-transformers/all-MiniLM-L6-v2")
    embeddings = model.encode(CANARIES, normalize_embeddings=True).tolist()
    out = [{"text": t, "embedding": e} for t, e in zip(CANARIES, embeddings)]
    json.dump(out, sys.stdout, indent=2)
    sys.stdout.write("\n")

if __name__ == "__main__":
    main()
