#!/usr/bin/env python3
"""Generate reference embeddings for the Go parity test.

Usage:
    pip install sentence-transformers
    scripts/gen-parity-fixture.py > internal/embed/local/testdata/parity.json

The resulting JSON has two sections:
  canaries: a list of {text, embedding} — the pure-Go encoder must match
    each embedding within 1e-4 cosine distance.
  ranking:  a list of (query, positive, negative) triples with each side's
    reference embedding. The Go test asserts that cosine(query, positive)
    > cosine(query, negative) both for the reference embeddings (sanity
    check) and for our encoder's embeddings (behavioral parity even when
    absolute vectors drift).
"""

import json
import sys

from sentence_transformers import SentenceTransformer

CANARIES = [
    # --- baseline ---
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

    # --- adversarial / boundary ---
    "   leading and trailing whitespace   ",
    "\t\ttabs and newlines\n\nand blank lines\n\n",
    "CASE case CaSe MiXeD — lowercasing is a preprocessing step",
    "Repeated words words words words words words words words",
    "1234567890 digits only string for tokenization sanity",
    "Hyphenated-words, under_scored_ident, dotted.path.names",
    "Emoji 🚀 mixed with text and more text 🌍 spanning tokens",
    "Near token limit: " + "word " * 120,
    "Well past token limit: " + "token " * 600,  # >512 subwords → truncation
    "単語で区切らない日本語の文章で位置エンコーディングを試す。" * 3,
    "العربية: هذا نص اختبار للتحقق من دعم النصوص من اليمين إلى اليسار.",
    "Lexical near-miss: Python programming versus Rust systems coding",
    "Query form: 'what is retrieval-augmented generation?'",
    "Passage form: 'Retrieval-augmented generation combines dense retrieval with a generative model.'",
]

# Ranking pairs: (query, positive passage, negative passage). The reference
# model scores query·positive > query·negative. We replicate this ordering
# test in Go to catch regressions where absolute cosine stays close but
# relative ordering flips (which would silently degrade search quality).
RANKING = [
    ("Paris is the capital of France.",
     "The capital city of France is Paris.",
     "Berlin is the capital of Germany."),
    ("memory safety in systems programming",
     "Rust enforces memory safety at compile time through ownership and borrowing.",
     "Pandas is a data analysis library in Python."),
    ("how to sort a list in python",
     "In Python, use list.sort() or the sorted() built-in function.",
     "Goroutines let you run concurrent workloads in Go."),
    ("machine learning for image classification",
     "Convolutional neural networks achieve high accuracy on image classification benchmarks.",
     "The Mediterranean diet emphasizes olive oil, vegetables, and fish."),
    ("git merge conflict resolution",
     "When branches diverge, git reports conflict markers that you edit before committing.",
     "The weather in San Francisco is typically mild and foggy in summer."),
]


def main() -> None:
    model = SentenceTransformer("sentence-transformers/all-MiniLM-L6-v2")
    all_texts = list(CANARIES)
    for q, p, n in RANKING:
        all_texts.extend([q, p, n])
    embeddings = model.encode(all_texts, normalize_embeddings=True).tolist()

    emb_iter = iter(embeddings)
    canary_out = [{"text": t, "embedding": next(emb_iter)} for t in CANARIES]
    ranking_out = []
    for q, p, n in RANKING:
        ranking_out.append({
            "query": q, "positive": p, "negative": n,
            "query_embedding": next(emb_iter),
            "positive_embedding": next(emb_iter),
            "negative_embedding": next(emb_iter),
        })
    json.dump({"canaries": canary_out, "ranking": ranking_out},
              sys.stdout, indent=2)
    sys.stdout.write("\n")


if __name__ == "__main__":
    main()
