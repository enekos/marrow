<script lang="ts">
  import { onMount } from 'svelte';
  import { Database, Search, Zap, GitBranch, Copy, Check, ArrowRight, ChevronDown, Settings, TerminalSquare, RefreshCw, Terminal, Code } from 'lucide-svelte';

  // Scramble utils
  const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789@#$%&*+<>{}[]_-\\/';
  function scrambleTo(target: string, progress: number) {
    return target.split('').map((ch, i) => {
      if (ch === ' ') return ' ';
      if (progress > i) return ch;
      return chars[Math.floor(Math.random() * chars.length)];
    }).join('');
  }
  function shuffleWords(text: string) {
    const words = text.split(' ');
    for (let i = words.length - 1; i > 0; i--) {
      const j = Math.floor(Math.random() * (i + 1));
      [words[i], words[j]] = [words[j], words[i]];
    }
    return words.join(' ');
  }

  // Hero text states
  let heroTitle = $state('');
  let heroSubtitle = $state('');
  const finalTitle = 'Local-first';
  const finalSubtitle = 'hybrid search for Markdown.';

  onMount(() => {
    // Initial word-shuffle chaos for subtitle
    let shuffles = 0;
    const maxShuffles = 10;
    const shuffleInterval = setInterval(() => {
      heroSubtitle = shuffleWords(finalSubtitle);
      shuffles++;
      if (shuffles >= maxShuffles) {
        clearInterval(shuffleInterval);
        heroSubtitle = finalSubtitle;
      }
    }, 80);

    // Character scramble for title
    let t = 0;
    const titleInterval = setInterval(() => {
      heroTitle = scrambleTo(finalTitle, t);
      t += 0.4;
      if (t >= finalTitle.length + 2) {
        clearInterval(titleInterval);
        heroTitle = finalTitle;
      }
    }, 40);
  });

  // Section heading scrambles
  let visibleSections = $state<Record<string, boolean>>({});
  function observeSection(id: string) {
    const el = document.getElementById(id);
    if (!el) return;
    const io = new IntersectionObserver((entries) => {
      entries.forEach((entry) => {
        if (entry.isIntersecting) {
          visibleSections[id] = true;
          io.disconnect();
        }
      });
    }, { threshold: 0.2 });
    io.observe(el);
  }
  onMount(() => {
    ['features', 'docs', 'search'].forEach(observeSection);
  });

  function useScramble(finalText: string, activeFn: () => boolean) {
    let text = $state('');
    $effect(() => {
      if (!activeFn()) { text = finalText; return; }
      let t = 0;
      const interval = setInterval(() => {
        text = scrambleTo(finalText, t);
        t += 0.5;
        if (t >= finalText.length + 2) {
          clearInterval(interval);
          text = finalText;
        }
      }, 35);
      return () => clearInterval(interval);
    });
    return () => text;
  }

  const featuresText = useScramble('Built for speed & privacy', () => !!visibleSections['features']);
  const docsText = useScramble('Documentation', () => !!visibleSections['docs']);
  const searchText = useScramble('Search your index', () => !!visibleSections['search']);

  // Terminal animation
  let visibleLines = $state(0);
  const terminalLines = [
    { text: '> marrow sync -dir ./docs -db marrow.db', delay: 0 },
    { text: '[marrow] indexing 124 markdown files...', delay: 900 },
    { text: '[marrow] sync complete. 124 docs indexed.', delay: 1900, success: true },
    { text: '> marrow serve -db marrow.db -addr :8080', delay: 2800 },
    { text: '[marrow] server listening on :8080', delay: 3700, success: true },
  ];
  onMount(() => {
    terminalLines.forEach((line, index) => {
      setTimeout(() => { visibleLines = index + 1; }, line.delay);
    });
  });

  // Background glyphs
  const glyphs = Array.from({ length: 40 }, () => ({
    char: chars[Math.floor(Math.random() * chars.length)],
    left: Math.random() * 100,
    top: Math.random() * 100,
    delay: Math.random() * 5,
    duration: 4 + Math.random() * 6,
    size: 10 + Math.floor(Math.random() * 18),
  }));

  // Copy install command
  let copied = $state(false);
  const installCmd = 'curl -sSL https://raw.githubusercontent.com/enekos/marrow/main/install.sh | sh';
  function copyCmd() {
    navigator.clipboard.writeText(installCmd);
    copied = true;
    setTimeout(() => copied = false, 1500);
  }

  // Search functionality
  let query = $state('');
  let sourceFilter = $state('');
  let docTypeFilter = $state('');
  let sources = $state<string[]>([]);
  let results: any[] = $state([]);
  let searching = $state(false);
  let searchError = $state('');

  onMount(() => {
    fetch('/stats')
      .then(r => r.json())
      .then(data => { sources = data.Sources || []; })
      .catch(() => {});
  });

  async function doSearch() {
    if (!query.trim()) return;
    searching = true;
    searchError = '';
    try {
      const body = { q: query, limit: 10, source: sourceFilter, doc_type: docTypeFilter };
      const r = await fetch('/search', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
      if (!r.ok) throw new Error('Search failed');
      const data = await r.json();
      results = data.results || [];
    } catch (e) {
      searchError = 'Search failed. Make sure the Marrow server is running.';
      results = [];
    } finally {
      searching = false;
    }
  }
  function handleKey(e: KeyboardEvent) { if (e.key === 'Enter') doSearch(); }

  // Accordion state
  let openSection = $state<string | null>('architecture');
  function toggleSection(id: string) { openSection = openSection === id ? null : id; }

  // Hover scramble for card titles
  function hoverScramble(node: HTMLElement, finalText: string) {
    let interval: ReturnType<typeof setInterval> | null = null;
    const enter = () => {
      let t = 0;
      interval = setInterval(() => {
        node.textContent = scrambleTo(finalText, t);
        t += 0.6;
        if (t >= finalText.length + 2) {
          if (interval) clearInterval(interval);
          node.textContent = finalText;
        }
      }, 30);
    };
    const leave = () => {
      if (interval) clearInterval(interval);
      node.textContent = finalText;
    };
    node.addEventListener('mouseenter', enter);
    node.addEventListener('mouseleave', leave);
    return {
      destroy() {
        node.removeEventListener('mouseenter', enter);
        node.removeEventListener('mouseleave', leave);
        if (interval) clearInterval(interval);
      }
    };
  }
</script>

<div class="relative min-h-screen text-[#e6e6eb] overflow-x-hidden font-sans bg-[#0b0a10]">
  <!-- Floating glyphs -->
  <div class="fixed inset-0 pointer-events-none z-0 overflow-hidden">
    {#each glyphs as g, i (i)}
      <div
        class="absolute text-[#ff4d6d]/10 font-mono select-none"
        style="left: {g.left}%; top: {g.top}%; font-size: {g.size}px; animation: float {g.duration}s ease-in-out infinite; animation-delay: -{g.delay}s;"
      >{g.char}</div>
    {/each}
  </div>

  <!-- Background orbs -->
  <div class="fixed inset-0 pointer-events-none z-0 overflow-hidden opacity-50">
    <div class="absolute top-[10%] left-[15%] w-[35vw] h-[35vw] rounded-full bg-[#ff4d6d] blur-[120px] opacity-20 animate-float"></div>
    <div class="absolute top-[35%] right-[10%] w-[40vw] h-[40vw] rounded-full bg-[#7c3aed] blur-[140px] opacity-15 animate-float" style="animation-delay: -4s;"></div>
    <div class="absolute -bottom-[10%] left-[30%] w-[45vw] h-[45vw] rounded-full bg-[#ff4d6d] blur-[120px] opacity-10 animate-float" style="animation-delay: -2s;"></div>
  </div>

  <!-- Nav -->
  <nav class="relative z-20 flex justify-between items-center px-6 md:px-12 py-6 bg-transparent">
    <div class="flex items-center gap-3 group cursor-pointer">
      <Database class="text-[#ff4d6d] group-hover:rotate-12 transition-transform duration-300" size={26} />
      <span class="text-xl font-bold text-white tracking-tight group-hover:tracking-widest transition-all duration-300">Marrow</span>
    </div>
    <a href="https://github.com/enekos/marrow" target="_blank" class="px-4 py-2 border border-white/10 rounded-full text-white hover:bg-white/10 hover:border-[#ff4d6d]/50 transition-all duration-300 text-sm flex items-center gap-2">
      <GitBranch size={14} /> GitHub
    </a>
  </nav>

  <!-- Hero -->
  <main class="relative z-10 max-w-6xl mx-auto px-6 md:px-12 pt-16 pb-24">
    <div class="flex flex-col lg:flex-row items-center justify-between gap-16">
      <div class="flex-1 space-y-6">
        <div class="inline-flex items-center gap-2 px-3 py-1 border border-[#ff4d6d]/30 rounded-full bg-[#ff4d6d]/10 text-[#ff4d6d] text-xs font-semibold tracking-wide uppercase animate-pulse">
          <Zap size={12} /> Open Source
        </div>
        <h1 class="text-4xl md:text-6xl font-bold leading-[1.05] text-white min-h-[1.2em]">
          <span class="inline-block min-w-[6ch]">{heroTitle}</span> <br/>
          <span class="bg-clip-text text-transparent bg-gradient-to-r from-[#ff4d6d] to-[#7c3aed]">hybrid search</span>
          <br/>
          <span class="inline-block min-w-[12ch]">{heroSubtitle}</span>
        </h1>
        <p class="text-lg text-[#a0a0b0] max-w-xl leading-relaxed">
          Marrow combines full-text search (FTS5) with vector similarity (sqlite-vec) inside a single SQLite file. Index your docs, issues, and pull requests—then search them instantly, entirely offline.
        </p>
        <div class="flex flex-col sm:flex-row gap-4 pt-4">
          <a href="#search" class="px-6 py-3 bg-[#ff4d6d] text-white rounded-full font-semibold text-sm transition-all hover:shadow-[0_0_25px_rgba(255,77,109,0.4)] hover:-translate-y-0.5 flex items-center justify-center gap-2">
            Try it <ArrowRight size={16} />
          </a>
          <button onclick={copyCmd} class="px-5 py-3 border border-white/10 rounded-full text-[#a0a0b0] text-sm flex items-center justify-center gap-2 bg-white/5 hover:bg-white/10 hover:border-white/20 transition-all font-mono">
            <span class="text-[#ff4d6d]">$</span> curl -sSL ...install.sh | sh
            {#if copied}
              <Check size={14} class="text-green-400" />
            {:else}
              <Copy size={14} />
            {/if}
          </button>
        </div>
      </div>

      <div class="flex-1 w-full max-w-lg relative">
        <div class="rounded-xl bg-[#121018]/90 backdrop-blur-xl border border-white/5 overflow-hidden shadow-2xl hover:shadow-[0_0_40px_rgba(255,77,109,0.15)] transition-shadow duration-500">
          <div class="flex items-center px-4 py-3 bg-black/30 border-b border-white/5">
            <div class="flex space-x-2">
              <div class="w-3 h-3 rounded-full bg-[#ff4d6d]/80"></div>
              <div class="w-3 h-3 rounded-full bg-white/20"></div>
              <div class="w-3 h-3 rounded-full bg-white/20"></div>
            </div>
            <div class="mx-auto text-xs text-[#7a7a8a] font-mono">marrow</div>
          </div>
          <div class="p-5 font-mono text-[13px] h-64 overflow-y-auto leading-relaxed custom-scrollbar">
            {#each terminalLines.slice(0, visibleLines) as line}
              <div class="mb-2 terminal-line">
                {#if line.text.startsWith('>')}
                  <span class="text-[#ff4d6d]">~</span> <span class="text-white">{line.text.substring(2)}</span>
                {:else if line.success}
                  <span class="text-white font-semibold">{line.text}</span>
                {:else}
                  <span class="text-[#7a7a8a]">{line.text}</span>
                {/if}
              </div>
            {/each}
            <div class="flex items-center text-white mt-1">
              <span class="text-[#ff4d6d]">~</span>&nbsp;<span class="animate-pulse">█</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </main>

  <!-- Feature cards -->
  <section id="features" class="relative z-10 py-20 border-y border-white/5 bg-[#0f0e14]">
    <div class="max-w-6xl mx-auto px-6">
      <div class="text-center mb-16">
        <h2 class="text-3xl md:text-4xl font-bold text-white mb-4 min-h-[1.2em]">{featuresText()}</h2>
        <p class="text-[#a0a0b0] max-w-2xl mx-auto">Everything runs locally in a single SQLite file. No external APIs required unless you explicitly enable them.</p>
      </div>
      <div class="grid md:grid-cols-2 lg:grid-cols-4 gap-6">
        <div class="group bg-[#13121a] border border-white/5 p-6 rounded-xl hover:border-[#ff4d6d]/40 transition-all hover:-translate-y-1 hover:shadow-[0_0_20px_rgba(255,77,109,0.1)]">
          <Search class="text-[#ff4d6d] mb-4 group-hover:scale-110 transition-transform" size={28} />
          <h3 class="text-lg font-semibold text-white mb-2" use:hoverScramble={'Hybrid Search'}>Hybrid Search</h3>
          <p class="text-[#9090a0] text-sm leading-relaxed">70% FTS5 full-text (BM25) + 30% sqlite-vec vector similarity fused with Reciprocal Rank Fusion, title boost, exact phrase boost, and recency decay.</p>
        </div>
        <div class="group bg-[#13121a] border border-white/5 p-6 rounded-xl hover:border-[#7c3aed]/40 transition-all hover:-translate-y-1 hover:shadow-[0_0_20px_rgba(124,58,237,0.1)]">
          <Database class="text-[#7c3aed] mb-4 group-hover:scale-110 transition-transform" size={28} />
          <h3 class="text-lg font-semibold text-white mb-2" use:hoverScramble={'Single SQLite File'}>Single SQLite File</h3>
          <p class="text-[#9090a0] text-sm leading-relaxed">Documents, FTS5 index, and 384-dim vectors all live in one portable <code>.db</code> file. Easy to back up, move, or version.</p>
        </div>
        <div class="group bg-[#13121a] border border-white/5 p-6 rounded-xl hover:border-[#ff4d6d]/40 transition-all hover:-translate-y-1 hover:shadow-[0_0_20px_rgba(255,77,109,0.1)]">
          <GitBranch class="text-[#ff4d6d] mb-4 group-hover:scale-110 transition-transform" size={28} />
          <h3 class="text-lg font-semibold text-white mb-2" use:hoverScramble={'GitHub Sync'}>GitHub Sync</h3>
          <p class="text-[#9090a0] text-sm leading-relaxed">Index live issues and pull requests via GitHub App webhooks. Closed items are automatically removed from the index.</p>
        </div>
        <div class="group bg-[#13121a] border border-white/5 p-6 rounded-xl hover:border-[#7c3aed]/40 transition-all hover:-translate-y-1 hover:shadow-[0_0_20px_rgba(124,58,237,0.1)]">
          <RefreshCw class="text-[#7c3aed] mb-4 group-hover:scale-110 transition-transform" size={28} />
          <h3 class="text-lg font-semibold text-white mb-2" use:hoverScramble={'Incremental Indexing'}>Incremental Indexing</h3>
          <p class="text-[#9090a0] text-sm leading-relaxed">Only changed files are re-indexed. Deleted files are automatically removed. Fast syncs for large repos.</p>
        </div>
      </div>
    </div>
  </section>

  <!-- Deep documentation -->
  <section id="docs" class="relative z-10 py-24">
    <div class="max-w-4xl mx-auto px-6">
      <div class="text-center mb-12">
        <h2 class="text-3xl md:text-4xl font-bold text-white mb-3 min-h-[1.2em]">{docsText()}</h2>
        <p class="text-[#a0a0b0]">How Marrow works, how to configure it, and how to use every feature.</p>
      </div>

      <!-- Accordion item: Architecture -->
      <div class="border border-white/5 rounded-xl overflow-hidden mb-4 bg-[#13121a]">
        <button onclick={() => toggleSection('architecture')} class="w-full flex items-center justify-between px-6 py-5 hover:bg-white/5 transition-colors">
          <div class="flex items-center gap-3 text-white font-semibold">
            <TerminalSquare size={20} class="text-[#ff4d6d]" /> Architecture
          </div>
          <ChevronDown size={18} class="text-[#7a7a8a] transition-transform {openSection === 'architecture' ? 'rotate-180' : ''}" />
        </button>
        {#if openSection === 'architecture'}
          <div class="px-6 pb-6 text-[#b0b0c0] text-sm leading-relaxed space-y-4">
            <p>Marrow stores everything in a single SQLite database with three core tables:</p>
            <ul class="list-disc pl-5 space-y-1">
              <li><strong class="text-white">documents</strong> — metadata (path, hash, title, lang, source, doc_type).</li>
              <li><strong class="text-white">documents_fts</strong> — a virtual FTS5 table for full-text search using the <code>unicode61</code> tokenizer.</li>
              <li><strong class="text-white">documents_vec</strong> — a virtual sqlite-vec table storing 384-dimensional float vectors.</li>
            </ul>
            <p>When you sync, Marrow parses Markdown with <code>goldmark</code>, extracts YAML frontmatter (title and lang), strips code blocks to reduce noise, and generates a SHA-256 content hash. Only files with new hashes are re-embedded and re-indexed.</p>
            <p>At search time, Marrow runs two independent queries—one against FTS5 (BM25 scoring) and one against sqlite-vec (cosine distance)—then merges the results with <strong class="text-white">Reciprocal Rank Fusion</strong> (RRF) using a 70/30 blend. Additional heuristics boost title matches, exact phrase matches, and recent documents.</p>
          </div>
        {/if}
      </div>

      <!-- Accordion item: Search Logic -->
      <div class="border border-white/5 rounded-xl overflow-hidden mb-4 bg-[#13121a]">
        <button onclick={() => toggleSection('searchlogic')} class="w-full flex items-center justify-between px-6 py-5 hover:bg-white/5 transition-colors">
          <div class="flex items-center gap-3 text-white font-semibold">
            <Search size={20} class="text-[#7c3aed]" /> Search Logic
          </div>
          <ChevronDown size={18} class="text-[#7a7a8a] transition-transform {openSection === 'searchlogic' ? 'rotate-180' : ''}" />
        </button>
        {#if openSection === 'searchlogic'}
          <div class="px-6 pb-6 text-[#b0b0c0] text-sm leading-relaxed space-y-4">
            <p>Search is not a simple keyword lookup. Marrow applies several layers of ranking:</p>
            <ol class="list-decimal pl-5 space-y-2">
              <li><strong class="text-white">Stemming & language detection</strong> — Queries are stemmed using pure-Go Snowball stemmers (English Porter2, Spanish, Basque). If no language hint is provided, Marrow auto-detects it from character patterns (e.g., ñ/¿/¡ for Spanish, tx/tz for Basque) and stopword frequencies.</li>
              <li><strong class="text-white">Dual retrieval</strong> — FTS5 returns BM25-ranked results; sqlite-vec returns cosine-similarity-ranked results. Both are limited to <code>limit × 3</code> candidates.</li>
              <li><strong class="text-white">RRF fusion</strong> — Each candidate gets a fused score: <code>70% FTS + 30% vector</code>, blending normalized rank positions and raw similarity scores.</li>
              <li><strong class="text-white">Title boost</strong> — If stemmed query tokens appear in the document title, the score is multiplied by up to <code>1.25×</code>.</li>
              <li><strong class="text-white">Exact phrase boost</strong> — Quoted phrases and title substring matches each give an extra <code>1.10×</code> multiplier.</li>
              <li><strong class="text-white">Recency boost</strong> — Documents modified within the last 180 days receive up to <code>1.05×</code>; older docs trend toward <code>1.0×</code>.</li>
            </ol>
            <p>You can filter results by <code>source</code>, <code>doc_type</code> (markdown, issue, pull_request), or <code>lang</code> via the API or the UI below.</p>
          </div>
        {/if}
      </div>

      <!-- Accordion item: Sync Sources -->
      <div class="border border-white/5 rounded-xl overflow-hidden mb-4 bg-[#13121a]">
        <button onclick={() => toggleSection('sync')} class="w-full flex items-center justify-between px-6 py-5 hover:bg-white/5 transition-colors">
          <div class="flex items-center gap-3 text-white font-semibold">
            <RefreshCw size={20} class="text-[#ff4d6d]" /> Sync Sources
          </div>
          <ChevronDown size={18} class="text-[#7a7a8a] transition-transform {openSection === 'sync' ? 'rotate-180' : ''}" />
        </button>
        {#if openSection === 'sync'}
          <div class="px-6 pb-6 text-[#b0b0c0] text-sm leading-relaxed space-y-4">
            <p>Marrow supports three source types, each with incremental sync behavior:</p>
            <ul class="list-disc pl-5 space-y-2">
              <li><strong class="text-white">Local directories</strong> — The watcher crawls the tree and uses <code>mtime</code> as a fast-path filter. If <code>mtime</code> hasn't changed since the last sync, the file is skipped. Otherwise, the file is read and its SHA-256 hash is compared to the stored hash. Deleted files are removed from the DB automatically.</li>
              <li><strong class="text-white">Git repositories</strong> — <code>gitpull.Sync</code> clones with <code>--depth 1</code> on first run, then fetches and hard-resets on subsequent runs. Only <code>.md</code> files that changed between the old and new HEAD are re-indexed.</li>
              <li><strong class="text-white">GitHub API (Issues & PRs)</strong> — Fetches open issues and pull requests, including comments. Each item gets a synthetic path like <code>gh:owner/repo/issues/123</code>. Webhooks keep the index live: opened/edited/reopened events re-index; closed events delete.</li>
            </ul>
            <p>Run <code>marrow sync</code> for a one-off sync, or use <code>marrow reindex</code> to force a full re-index across all configured sources.</p>
          </div>
        {/if}
      </div>

      <!-- Accordion item: Embeddings -->
      <div class="border border-white/5 rounded-xl overflow-hidden mb-4 bg-[#13121a]">
        <button onclick={() => toggleSection('embeddings')} class="w-full flex items-center justify-between px-6 py-5 hover:bg-white/5 transition-colors">
          <div class="flex items-center gap-3 text-white font-semibold">
            <Code size={20} class="text-[#7c3aed]" /> Embeddings
          </div>
          <ChevronDown size={18} class="text-[#7a7a8a] transition-transform {openSection === 'embeddings' ? 'rotate-180' : ''}" />
        </button>
        {#if openSection === 'embeddings'}
          <div class="px-6 pb-6 text-[#b0b0c0] text-sm leading-relaxed space-y-4">
            <p>Marrow abstracts embeddings behind a simple <code>Func</code> interface. Three providers are supported:</p>
            <ul class="list-disc pl-5 space-y-2">
              <li><strong class="text-white">mock</strong> (default) — Deterministic 384-dim vectors generated from the SHA-256 hash of the text via Box-Muller normal distribution. Zero-config, fully offline, and reproducible across runs. Great for CI and testing.</li>
              <li><strong class="text-white">ollama</strong> — Talks to a local Ollama server (default <code>http://localhost:11434</code>, model <code>nomic-embed-text</code>). Fully local, no API keys.</li>
              <li><strong class="text-white">openai</strong> — Talks to any OpenAI-compatible API (default <code>text-embedding-3-small</code>). Requires an <code>api_key</code>; optionally override <code>base_url</code> for proxies or Azure.</li>
            </ul>
            <p>Configure the provider in your config file or via environment variables: <code>MARROW_EMBEDDING_PROVIDER=ollama</code>, <code>MARROW_EMBEDDING_MODEL=...</code>, <code>MARROW_EMBEDDING_BASE_URL=...</code>, <code>MARROW_EMBEDDING_API_KEY=...</code>.</p>
          </div>
        {/if}
      </div>

      <!-- Accordion item: Configuration -->
      <div class="border border-white/5 rounded-xl overflow-hidden mb-4 bg-[#13121a]">
        <button onclick={() => toggleSection('config')} class="w-full flex items-center justify-between px-6 py-5 hover:bg-white/5 transition-colors">
          <div class="flex items-center gap-3 text-white font-semibold">
            <Settings size={20} class="text-[#ff4d6d]" /> Configuration
          </div>
          <ChevronDown size={18} class="text-[#7a7a8a] transition-transform {openSection === 'config' ? 'rotate-180' : ''}" />
        </button>
        {#if openSection === 'config'}
          <div class="px-6 pb-6 text-[#b0b0c0] text-sm leading-relaxed space-y-4">
            <p>Marrow uses a four-layer configuration cascade (later layers override earlier ones):</p>
            <ol class="list-decimal pl-5 space-y-1">
              <li>Hardcoded defaults</li>
              <li>User config: <code class="text-white">~/.config/marrow/config.toml</code></li>
              <li>Project config: <code class="text-white">.marrow.toml</code> (discovered by walking up from the working directory)</li>
              <li>Environment variables with the <code class="text-white">MARROW_</code> prefix</li>
            </ol>
            <p>Example <code>.marrow.toml</code>:</p>
            <pre class="bg-black/40 border border-white/5 rounded-lg p-4 font-mono text-xs text-[#a0a0b0] overflow-x-auto">[server]
db = "marrow.db"
addr = ":8080"

[search]
detect_lang = true
default_lang = "en"

[embedding]
provider = "ollama"
model = "nomic-embed-text"
base_url = "http://localhost:11434"

[[sources]]
name = "docs"
type = "local"
dir = "./docs"

[[sources]]
name = "wiki"
type = "git"
repo_url = "https://github.com/owner/wiki.git"
local_path = "./repo/wiki"</pre>
            <p>With multiple sources configured, <code>marrow sync --all</code> and <code>marrow serve</code> will index each source automatically.</p>
          </div>
        {/if}
      </div>

      <!-- Accordion item: CLI Commands -->
      <div class="border border-white/5 rounded-xl overflow-hidden mb-4 bg-[#13121a]">
        <button onclick={() => toggleSection('cli')} class="w-full flex items-center justify-between px-6 py-5 hover:bg-white/5 transition-colors">
          <div class="flex items-center gap-3 text-white font-semibold">
            <Terminal size={20} class="text-[#7c3aed]" /> CLI Commands
          </div>
          <ChevronDown size={18} class="text-[#7a7a8a] transition-transform {openSection === 'cli' ? 'rotate-180' : ''}" />
        </button>
        {#if openSection === 'cli'}
          <div class="px-6 pb-6 text-[#b0b0c0] text-sm leading-relaxed space-y-4">
            <ul class="space-y-3">
              <li>
                <code class="text-white bg-white/10 px-1.5 py-0.5 rounded">marrow sync</code>
                <p class="mt-1">One-off incremental sync of a local directory. Flags: <code>-dir</code>, <code>-db</code>, <code>-source</code>, <code>-default-lang</code>. Use <code>-all</code> to sync every configured source.</p>
              </li>
              <li>
                <code class="text-white bg-white/10 px-1.5 py-0.5 rounded">marrow serve</code>
                <p class="mt-1">Starts the HTTP API and serves this landing page. Flags include <code>-db</code>, <code>-addr</code>, <code>-detect-lang</code>, <code>-default-lang</code>, plus GitHub/GitHub App options for live repo/issue sync.</p>
              </li>
              <li>
                <code class="text-white bg-white/10 px-1.5 py-0.5 rounded">marrow status</code>
                <p class="mt-1">Prints database stats: total docs, size, last sync, breakdown by source and doc type.</p>
              </li>
              <li>
                <code class="text-white bg-white/10 px-1.5 py-0.5 rounded">marrow reindex</code>
                <p class="mt-1">Force a full reindex of all configured sources. Useful after schema changes or large refactors.</p>
              </li>
              <li>
                <code class="text-white bg-white/10 px-1.5 py-0.5 rounded">marrow maintain</code>
                <p class="mt-1">Runs VACUUM and prunes orphaned FTS/vector rows. Use <code>-backup</code> to create a timestamped backup first.</p>
              </li>
            </ul>
          </div>
        {/if}
      </div>

      <!-- Accordion item: API -->
      <div class="border border-white/5 rounded-xl overflow-hidden mb-4 bg-[#13121a]">
        <button onclick={() => toggleSection('api')} class="w-full flex items-center justify-between px-6 py-5 hover:bg-white/5 transition-colors">
          <div class="flex items-center gap-3 text-white font-semibold">
            <Code size={20} class="text-[#ff4d6d]" /> API Endpoints
          </div>
          <ChevronDown size={18} class="text-[#7a7a8a] transition-transform {openSection === 'api' ? 'rotate-180' : ''}" />
        </button>
        {#if openSection === 'api'}
          <div class="px-6 pb-6 text-[#b0b0c0] text-sm leading-relaxed space-y-4">
            <div class="space-y-3">
              <div>
                <div class="font-mono text-xs text-[#ff4d6d] mb-1">POST /search</div>
                <pre class="bg-black/40 border border-white/5 rounded-lg p-3 font-mono text-xs text-[#a0a0b0] overflow-x-auto">{'{'} "q": "authentication flow", "limit": 10, "lang": "en", "source": "docs", "doc_type": "markdown" {'}'}</pre>
                <p class="mt-1">Returns ranked results with <code>id</code>, <code>path</code>, <code>title</code>, <code>doc_type</code>, and <code>score</code>.</p>
              </div>
              <div>
                <div class="font-mono text-xs text-[#ff4d6d] mb-1">GET /stats</div>
                <p>Returns aggregate statistics: <code>TotalDocs</code>, <code>DBSizeBytes</code>, <code>Sources</code>, <code>BySource</code>, <code>ByDocType</code>, and <code>LastSyncAt</code>.</p>
              </div>
              <div>
                <div class="font-mono text-xs text-[#ff4d6d] mb-1">POST /webhook</div>
                <p>Accepts two webhook types:</p>
                <ul class="list-disc pl-5 space-y-1 mt-1">
                  <li><strong>Legacy</strong> — Trigger a background git re-sync by sending <code>X-Marrow-Secret</code>.</li>
                  <li><strong>GitHub App</strong> — Accepts real GitHub webhooks (issues, pull_request, issue_comment, pull_request_review_comment) signed with <code>X-Hub-Signature-256</code>.</li>
                </ul>
              </div>
              <div>
                <div class="font-mono text-xs text-[#ff4d6d] mb-1">GET /health</div>
                <p>Returns <code>{'{"status":"ok"}'}</code>.</p>
              </div>
            </div>
          </div>
        {/if}
      </div>
    </div>
  </section>

  <!-- Search UI -->
  <section id="search" class="relative z-10 py-24 border-y border-white/5 bg-[#0f0e14]">
    <div class="max-w-3xl mx-auto px-6">
      <div class="text-center mb-10">
        <h2 class="text-3xl md:text-4xl font-bold text-white mb-3 min-h-[1.2em]">{searchText()}</h2>
        <p class="text-[#a0a0b0]">Try a live query against the currently running Marrow instance.</p>
      </div>

      <div class="bg-[#13121a] border border-white/5 rounded-xl p-6 shadow-xl">
        <div class="flex flex-col md:flex-row gap-3 mb-4">
          <input
            type="text"
            bind:value={query}
            onkeydown={handleKey}
            placeholder="Search..."
            class="flex-1 px-4 py-3 bg-black/30 border border-white/10 rounded-lg text-white placeholder-[#606070] focus:outline-none focus:border-[#ff4d6d]/50"
          />
          <select
            bind:value={sourceFilter}
            class="px-4 py-3 bg-black/30 border border-white/10 rounded-lg text-white focus:outline-none focus:border-[#ff4d6d]/50"
          >
            <option value="">All sources</option>
            {#each sources as src}
              <option value={src}>{src}</option>
            {/each}
          </select>
          <select
            bind:value={docTypeFilter}
            class="px-4 py-3 bg-black/30 border border-white/10 rounded-lg text-white focus:outline-none focus:border-[#ff4d6d]/50"
          >
            <option value="">All types</option>
            <option value="markdown">markdown</option>
            <option value="issue">issue</option>
            <option value="pull_request">pull_request</option>
          </select>
        </div>
        <button
          onclick={doSearch}
          class="w-full md:w-auto px-6 py-3 bg-[#ff4d6d] text-white rounded-lg font-semibold text-sm hover:bg-[#ff3355] transition-colors flex items-center justify-center gap-2"
        >
          {#if searching}
            <span class="inline-block w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin"></span>
            Searching...
          {:else}
            <Search size={16} /> Search
          {/if}
        </button>

        {#if searchError}
          <div class="mt-4 text-red-400 text-sm">{searchError}</div>
        {/if}

        {#if results.length > 0}
          <div class="mt-6 space-y-3">
            {#each results as res}
              <div class="p-4 bg-black/20 border border-white/5 rounded-lg hover:border-white/10 transition-colors">
                <h3 class="text-white font-semibold mb-1">{res.title || res.path}</h3>
                <div class="text-xs text-[#808090]">
                  {res.doc_type} &middot; {res.path} &middot; score {typeof res.score === 'number' ? res.score.toFixed(3) : res.score}
                </div>
              </div>
            {/each}
          </div>
        {:else if !searching && query && !searchError}
          <div class="mt-6 text-[#707080] text-sm">No results found.</div>
        {/if}
      </div>
    </div>
  </section>

  <!-- Footer -->
  <footer class="relative z-10 py-10 bg-[#0b0a10]">
    <div class="max-w-6xl mx-auto px-6 flex flex-col md:flex-row justify-between items-center gap-4 text-[#707080] text-sm">
      <div class="flex items-center gap-2 font-semibold text-white">
        <Database size={18} class="text-[#ff4d6d]" /> Marrow
      </div>
      <div>
        Built by <a href="https://github.com/enekos" class="text-[#a0a0b0] hover:text-white transition-colors">enekos</a>
      </div>
    </div>
  </footer>
</div>
