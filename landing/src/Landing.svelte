<script lang="ts">
  import { onMount } from 'svelte';
  import { Database, Search, Zap, GitBranch, Copy, Check, ArrowRight, ChevronDown, Settings, TerminalSquare, RefreshCw, Terminal, Code, FileText, Lock, Sparkles, Users, BookOpen, FolderTree, Layers } from 'lucide-svelte';

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
  const finalTitle = 'Find anything';
  const finalSubtitle = 'in your Markdown.';

  onMount(() => {
    let shuffles = 0;
    const maxShuffles = 8;
    const shuffleInterval = setInterval(() => {
      heroSubtitle = shuffleWords(finalSubtitle);
      shuffles++;
      if (shuffles >= maxShuffles) {
        clearInterval(shuffleInterval);
        heroSubtitle = finalSubtitle;
      }
    }, 80);

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
    ['what', 'how', 'features', 'use-cases', 'quickstart', 'docs', 'search'].forEach(observeSection);
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

  const whatText = useScramble('What is Marrow?', () => !!visibleSections['what']);
  const howText = useScramble('How it works', () => !!visibleSections['how']);
  const featuresText = useScramble('Built for speed & privacy', () => !!visibleSections['features']);
  const useCasesText = useScramble('Who it’s for', () => !!visibleSections['use-cases']);
  const quickstartText = useScramble('Quick start', () => !!visibleSections['quickstart']);
  const docsText = useScramble('Documentation', () => !!visibleSections['docs']);
  const searchText = useScramble('Try it live', () => !!visibleSections['search']);

  // Terminal animation
  let visibleLines = $state(0);
  const terminalLines = [
    { text: '> marrow sync -dir ./docs', delay: 0 },
    { text: '[marrow] indexing 124 markdown files...', delay: 800 },
    { text: '[marrow] sync complete. 124 docs indexed.', delay: 1700, success: true },
    { text: '> marrow serve -addr :8080', delay: 2500 },
    { text: '[marrow] server listening on :8080', delay: 3300, success: true },
  ];
  onMount(() => {
    terminalLines.forEach((line, index) => {
      setTimeout(() => { visibleLines = index + 1; }, line.delay);
    });
  });

  // Background glyphs (fewer on mobile via CSS)
  const glyphs = Array.from({ length: 30 }, () => ({
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
  function copyCmd(cmd?: string) {
    navigator.clipboard.writeText(cmd ?? installCmd);
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
  let searchAvailable = $state<boolean | null>(null);

  onMount(() => {
    fetch('/stats')
      .then(r => r.ok ? r.json() : Promise.reject())
      .then(data => { sources = data.Sources || []; searchAvailable = true; })
      .catch(() => { searchAvailable = false; });
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
  <!-- Floating glyphs (hidden on small screens to reduce clutter) -->
  <div class="fixed inset-0 pointer-events-none z-0 overflow-hidden hidden sm:block">
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
  <nav class="relative z-20 flex justify-between items-center px-4 sm:px-6 md:px-12 py-5 sm:py-6 bg-transparent">
    <div class="flex items-center gap-2 sm:gap-3 group cursor-pointer">
      <Database class="text-[#ff4d6d] group-hover:rotate-12 transition-transform duration-300" size={24} />
      <span class="text-lg sm:text-xl font-bold text-white tracking-tight group-hover:tracking-widest transition-all duration-300">Marrow</span>
    </div>
    <div class="flex items-center gap-2 sm:gap-3">
      <a href="#quickstart" class="hidden sm:inline-flex px-4 py-2 text-sm text-[#a0a0b0] hover:text-white transition-colors">Quick start</a>
      <a href="#docs" class="hidden sm:inline-flex px-4 py-2 text-sm text-[#a0a0b0] hover:text-white transition-colors">Docs</a>
      <a href="https://github.com/enekos/marrow" target="_blank" rel="noopener" class="px-3 sm:px-4 py-2 border border-white/10 rounded-full text-white hover:bg-white/10 hover:border-[#ff4d6d]/50 transition-all duration-300 text-xs sm:text-sm flex items-center gap-2">
        <GitBranch size={14} /> GitHub
      </a>
    </div>
  </nav>

  <!-- Hero -->
  <main class="relative z-10 max-w-6xl mx-auto px-4 sm:px-6 md:px-12 pt-10 sm:pt-16 pb-16 sm:pb-24">
    <div class="flex flex-col lg:flex-row items-center justify-between gap-10 lg:gap-16">
      <div class="flex-1 space-y-5 sm:space-y-6 text-center lg:text-left">
        <div class="inline-flex items-center gap-2 px-3 py-1 border border-[#ff4d6d]/30 rounded-full bg-[#ff4d6d]/10 text-[#ff4d6d] text-xs font-semibold tracking-wide uppercase">
          <Zap size={12} /> Open source · Local-first
        </div>
        <h1 class="text-[2.25rem] sm:text-5xl md:text-6xl font-bold leading-[1.05] text-white">
          <span class="inline-block min-w-[7ch]">{heroTitle}</span> <br/>
          <span class="bg-clip-text text-transparent bg-gradient-to-r from-[#ff4d6d] to-[#7c3aed]">in your Markdown</span>
          <br/>
          <span class="inline-block min-w-[12ch] text-white/90 text-3xl sm:text-4xl md:text-5xl">{heroSubtitle}</span>
        </h1>
        <p class="text-base sm:text-lg text-[#a0a0b0] max-w-xl mx-auto lg:mx-0 leading-relaxed">
          Marrow is a tiny search engine you run on your own machine. Point it at a folder of notes, docs, or a GitHub repo — then search by keyword <em>or</em> by meaning. Everything lives in one SQLite file. No cloud, no account, no telemetry.
        </p>
        <div class="flex flex-col sm:flex-row gap-3 sm:gap-4 pt-2 justify-center lg:justify-start">
          <a href="#quickstart" class="px-6 py-3 bg-[#ff4d6d] text-white rounded-full font-semibold text-sm transition-all hover:shadow-[0_0_25px_rgba(255,77,109,0.4)] hover:-translate-y-0.5 flex items-center justify-center gap-2">
            Get started <ArrowRight size={16} />
          </a>
          <button onclick={() => copyCmd()} class="px-4 sm:px-5 py-3 border border-white/10 rounded-full text-[#a0a0b0] text-xs sm:text-sm flex items-center justify-center gap-2 bg-white/5 hover:bg-white/10 hover:border-white/20 transition-all font-mono max-w-full overflow-hidden">
            <span class="text-[#ff4d6d] shrink-0">$</span>
            <span class="truncate">curl -sSL .../install.sh | sh</span>
            {#if copied}
              <Check size={14} class="text-green-400 shrink-0" />
            {:else}
              <Copy size={14} class="shrink-0" />
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
          <div class="p-4 sm:p-5 font-mono text-[11px] sm:text-[13px] h-56 sm:h-64 overflow-y-auto leading-relaxed custom-scrollbar">
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

  <!-- What is Marrow? (plain English) -->
  <section id="what" class="relative z-10 py-16 sm:py-20 border-y border-white/5 bg-[#0f0e14]">
    <div class="max-w-4xl mx-auto px-4 sm:px-6">
      <div class="text-center mb-10 sm:mb-14">
        <h2 class="text-2xl sm:text-3xl md:text-4xl font-bold text-white mb-3 min-h-[1.2em]">{whatText()}</h2>
        <p class="text-[#a0a0b0] max-w-2xl mx-auto text-sm sm:text-base">A short, no-jargon explanation.</p>
      </div>

      <div class="grid sm:grid-cols-2 gap-5 sm:gap-6">
        <div class="bg-[#13121a] border border-white/5 rounded-xl p-5 sm:p-6">
          <h3 class="text-white font-semibold mb-2 flex items-center gap-2"><Search size={18} class="text-[#ff4d6d]" /> The problem</h3>
          <p class="text-[#a0a0b0] text-sm leading-relaxed">
            Plain keyword search misses synonyms. Semantic / AI search misses exact terms and feels slow. And most tools ship your private notes off to a third-party server.
          </p>
        </div>
        <div class="bg-[#13121a] border border-white/5 rounded-xl p-5 sm:p-6">
          <h3 class="text-white font-semibold mb-2 flex items-center gap-2"><Sparkles size={18} class="text-[#7c3aed]" /> The fix</h3>
          <p class="text-[#a0a0b0] text-sm leading-relaxed">
            Marrow runs <strong class="text-white">two searches at once</strong> — exact-match keyword <em>and</em> meaning-based — then merges the results. Same speed as a normal search, but it finds what you meant, not just what you typed. All offline.
          </p>
        </div>
      </div>

      <div class="mt-6 sm:mt-8 bg-gradient-to-br from-[#ff4d6d]/10 to-[#7c3aed]/10 border border-white/10 rounded-xl p-5 sm:p-6 text-center">
        <p class="text-[#d0d0e0] text-sm sm:text-base leading-relaxed max-w-2xl mx-auto">
          Search <code class="text-[#ff4d6d]">"how to log in"</code> and Marrow returns your notes about <em>"authentication"</em>, <em>"OAuth setup"</em>, and <em>"session cookies"</em> — even if those exact words never appear in the query.
        </p>
      </div>
    </div>
  </section>

  <!-- How it works -->
  <section id="how" class="relative z-10 py-16 sm:py-20">
    <div class="max-w-5xl mx-auto px-4 sm:px-6">
      <div class="text-center mb-10 sm:mb-14">
        <h2 class="text-2xl sm:text-3xl md:text-4xl font-bold text-white mb-3 min-h-[1.2em]">{howText()}</h2>
        <p class="text-[#a0a0b0] max-w-2xl mx-auto text-sm sm:text-base">Three steps. No servers to set up, no accounts to create.</p>
      </div>

      <div class="grid md:grid-cols-3 gap-4 sm:gap-6">
        <div class="relative bg-[#13121a] border border-white/5 rounded-xl p-5 sm:p-6">
          <div class="absolute -top-3 -left-3 w-8 h-8 rounded-full bg-[#ff4d6d] text-white font-bold text-sm flex items-center justify-center shadow-lg">1</div>
          <FolderTree class="text-[#ff4d6d] mb-3" size={26} />
          <h3 class="text-white font-semibold mb-2">Point at your files</h3>
          <p class="text-[#9090a0] text-sm leading-relaxed">A folder of <code>.md</code> notes, a Git repo, or a GitHub project's issues and pull requests. Marrow walks the tree and reads each file.</p>
        </div>
        <div class="relative bg-[#13121a] border border-white/5 rounded-xl p-5 sm:p-6">
          <div class="absolute -top-3 -left-3 w-8 h-8 rounded-full bg-[#7c3aed] text-white font-bold text-sm flex items-center justify-center shadow-lg">2</div>
          <Layers class="text-[#7c3aed] mb-3" size={26} />
          <h3 class="text-white font-semibold mb-2">Indexed into one file</h3>
          <p class="text-[#9090a0] text-sm leading-relaxed">Each document gets a keyword index <em>and</em> a numeric "meaning fingerprint" (an embedding). Both are stored in a single portable SQLite <code>.db</code> file.</p>
        </div>
        <div class="relative bg-[#13121a] border border-white/5 rounded-xl p-5 sm:p-6">
          <div class="absolute -top-3 -left-3 w-8 h-8 rounded-full bg-[#ff4d6d] text-white font-bold text-sm flex items-center justify-center shadow-lg">3</div>
          <Search class="text-[#ff4d6d] mb-3" size={26} />
          <h3 class="text-white font-semibold mb-2">Search instantly</h3>
          <p class="text-[#9090a0] text-sm leading-relaxed">Hit a local HTTP endpoint or use the CLI. Results combine keyword and semantic matches, ranked together. Re-runs only re-index files that actually changed.</p>
        </div>
      </div>
    </div>
  </section>

  <!-- Feature cards -->
  <section id="features" class="relative z-10 py-16 sm:py-20 border-y border-white/5 bg-[#0f0e14]">
    <div class="max-w-6xl mx-auto px-4 sm:px-6">
      <div class="text-center mb-10 sm:mb-14">
        <h2 class="text-2xl sm:text-3xl md:text-4xl font-bold text-white mb-3 min-h-[1.2em]">{featuresText()}</h2>
        <p class="text-[#a0a0b0] max-w-2xl mx-auto text-sm sm:text-base">Everything runs on your machine. No external APIs unless you turn them on.</p>
      </div>
      <div class="grid sm:grid-cols-2 lg:grid-cols-4 gap-4 sm:gap-6">
        <div class="group bg-[#13121a] border border-white/5 p-5 sm:p-6 rounded-xl hover:border-[#ff4d6d]/40 transition-all hover:-translate-y-1 hover:shadow-[0_0_20px_rgba(255,77,109,0.1)]">
          <Search class="text-[#ff4d6d] mb-4 group-hover:scale-110 transition-transform" size={26} />
          <h3 class="text-base sm:text-lg font-semibold text-white mb-2" use:hoverScramble={'Hybrid search'}>Hybrid search</h3>
          <p class="text-[#9090a0] text-sm leading-relaxed">Keyword matching (BM25) <em>and</em> meaning-based matching (vectors) merged with Reciprocal Rank Fusion. Best of both, automatically.</p>
        </div>
        <div class="group bg-[#13121a] border border-white/5 p-5 sm:p-6 rounded-xl hover:border-[#7c3aed]/40 transition-all hover:-translate-y-1 hover:shadow-[0_0_20px_rgba(124,58,237,0.1)]">
          <Database class="text-[#7c3aed] mb-4 group-hover:scale-110 transition-transform" size={26} />
          <h3 class="text-base sm:text-lg font-semibold text-white mb-2" use:hoverScramble={'One SQLite file'}>One SQLite file</h3>
          <p class="text-[#9090a0] text-sm leading-relaxed">Documents, full-text index, and vectors all in one <code>.db</code>. Easy to back up, drop into Dropbox, or commit to a repo.</p>
        </div>
        <div class="group bg-[#13121a] border border-white/5 p-5 sm:p-6 rounded-xl hover:border-[#ff4d6d]/40 transition-all hover:-translate-y-1 hover:shadow-[0_0_20px_rgba(255,77,109,0.1)]">
          <Lock class="text-[#ff4d6d] mb-4 group-hover:scale-110 transition-transform" size={26} />
          <h3 class="text-base sm:text-lg font-semibold text-white mb-2" use:hoverScramble={'Truly local'}>Truly local</h3>
          <p class="text-[#9090a0] text-sm leading-relaxed">Default config never leaves your machine. Use Ollama for local embeddings, or plug in OpenAI only if you want to.</p>
        </div>
        <div class="group bg-[#13121a] border border-white/5 p-5 sm:p-6 rounded-xl hover:border-[#7c3aed]/40 transition-all hover:-translate-y-1 hover:shadow-[0_0_20px_rgba(124,58,237,0.1)]">
          <GitBranch class="text-[#7c3aed] mb-4 group-hover:scale-110 transition-transform" size={26} />
          <h3 class="text-base sm:text-lg font-semibold text-white mb-2" use:hoverScramble={'GitHub-aware'}>GitHub-aware</h3>
          <p class="text-[#9090a0] text-sm leading-relaxed">Optional GitHub App: index live issues and PRs. Webhooks keep the index up to date — closed items auto-removed.</p>
        </div>
        <div class="group bg-[#13121a] border border-white/5 p-5 sm:p-6 rounded-xl hover:border-[#ff4d6d]/40 transition-all hover:-translate-y-1 hover:shadow-[0_0_20px_rgba(255,77,109,0.1)]">
          <RefreshCw class="text-[#ff4d6d] mb-4 group-hover:scale-110 transition-transform" size={26} />
          <h3 class="text-base sm:text-lg font-semibold text-white mb-2" use:hoverScramble={'Incremental'}>Incremental</h3>
          <p class="text-[#9090a0] text-sm leading-relaxed">Only changed files are re-indexed. Big repos resync in seconds. Deleted files disappear automatically.</p>
        </div>
        <div class="group bg-[#13121a] border border-white/5 p-5 sm:p-6 rounded-xl hover:border-[#7c3aed]/40 transition-all hover:-translate-y-1 hover:shadow-[0_0_20px_rgba(124,58,237,0.1)]">
          <BookOpen class="text-[#7c3aed] mb-4 group-hover:scale-110 transition-transform" size={26} />
          <h3 class="text-base sm:text-lg font-semibold text-white mb-2" use:hoverScramble={'Multilingual'}>Multilingual</h3>
          <p class="text-[#9090a0] text-sm leading-relaxed">Built-in stemmers for English, Spanish, and Basque. Auto-detects the query language so plurals and conjugations still match.</p>
        </div>
        <div class="group bg-[#13121a] border border-white/5 p-5 sm:p-6 rounded-xl hover:border-[#ff4d6d]/40 transition-all hover:-translate-y-1 hover:shadow-[0_0_20px_rgba(255,77,109,0.1)]">
          <Zap class="text-[#ff4d6d] mb-4 group-hover:scale-110 transition-transform" size={26} />
          <h3 class="text-base sm:text-lg font-semibold text-white mb-2" use:hoverScramble={'Single binary'}>Single binary</h3>
          <p class="text-[#9090a0] text-sm leading-relaxed">No Docker, no Python, no Node runtime. One small Go executable. Install it with curl, run it from anywhere.</p>
        </div>
        <div class="group bg-[#13121a] border border-white/5 p-5 sm:p-6 rounded-xl hover:border-[#7c3aed]/40 transition-all hover:-translate-y-1 hover:shadow-[0_0_20px_rgba(124,58,237,0.1)]">
          <Code class="text-[#7c3aed] mb-4 group-hover:scale-110 transition-transform" size={26} />
          <h3 class="text-base sm:text-lg font-semibold text-white mb-2" use:hoverScramble={'Simple HTTP API'}>Simple HTTP API</h3>
          <p class="text-[#9090a0] text-sm leading-relaxed">A few JSON endpoints — search, stats, webhook. Drop a search box on your static site or wire it into your CLI tooling.</p>
        </div>
      </div>
    </div>
  </section>

  <!-- Use cases -->
  <section id="use-cases" class="relative z-10 py-16 sm:py-20">
    <div class="max-w-5xl mx-auto px-4 sm:px-6">
      <div class="text-center mb-10 sm:mb-14">
        <h2 class="text-2xl sm:text-3xl md:text-4xl font-bold text-white mb-3 min-h-[1.2em]">{useCasesText()}</h2>
        <p class="text-[#a0a0b0] max-w-2xl mx-auto text-sm sm:text-base">If you have a pile of Markdown, Marrow probably fits.</p>
      </div>

      <div class="grid sm:grid-cols-2 lg:grid-cols-3 gap-4 sm:gap-6">
        <div class="bg-[#13121a] border border-white/5 rounded-xl p-5 sm:p-6">
          <FileText class="text-[#ff4d6d] mb-3" size={24} />
          <h3 class="text-white font-semibold mb-1.5">Personal note vaults</h3>
          <p class="text-[#9090a0] text-sm leading-relaxed">Obsidian, Logseq, plain folders. Find half-remembered ideas without third-party AI services scraping your journal.</p>
        </div>
        <div class="bg-[#13121a] border border-white/5 rounded-xl p-5 sm:p-6">
          <BookOpen class="text-[#7c3aed] mb-3" size={24} />
          <h3 class="text-white font-semibold mb-1.5">Static site search</h3>
          <p class="text-[#9090a0] text-sm leading-relaxed">Hugo, Astro, Docusaurus, MkDocs. Drop a Marrow instance behind your docs and replace flaky client-side search with real ranking.</p>
        </div>
        <div class="bg-[#13121a] border border-white/5 rounded-xl p-5 sm:p-6">
          <Users class="text-[#ff4d6d] mb-3" size={24} />
          <h3 class="text-white font-semibold mb-1.5">Team wikis</h3>
          <p class="text-[#9090a0] text-sm leading-relaxed">Point Marrow at your internal docs repo and live GitHub issues. One search bar that knows about RFCs <em>and</em> ongoing tickets.</p>
        </div>
        <div class="bg-[#13121a] border border-white/5 rounded-xl p-5 sm:p-6">
          <Code class="text-[#7c3aed] mb-3" size={24} />
          <h3 class="text-white font-semibold mb-1.5">Codebase READMEs</h3>
          <p class="text-[#9090a0] text-sm leading-relaxed">A monorepo with hundreds of <code>README.md</code> files? Marrow gives you a unified search across every package's docs.</p>
        </div>
        <div class="bg-[#13121a] border border-white/5 rounded-xl p-5 sm:p-6">
          <Sparkles class="text-[#ff4d6d] mb-3" size={24} />
          <h3 class="text-white font-semibold mb-1.5">"Related articles"</h3>
          <p class="text-[#9090a0] text-sm leading-relaxed">Use the GitHub Action to generate a static <code>related.json</code> at build time, so your blog can link related posts without a runtime server.</p>
        </div>
        <div class="bg-[#13121a] border border-white/5 rounded-xl p-5 sm:p-6">
          <Terminal class="text-[#7c3aed] mb-3" size={24} />
          <h3 class="text-white font-semibold mb-1.5">CLI / agent tooling</h3>
          <p class="text-[#9090a0] text-sm leading-relaxed">A predictable JSON API for retrieval. Wire it into editor plugins, terminal helpers, or agent loops as a fast local knowledge base.</p>
        </div>
      </div>
    </div>
  </section>

  <!-- Quick start -->
  <section id="quickstart" class="relative z-10 py-16 sm:py-20 border-y border-white/5 bg-[#0f0e14]">
    <div class="max-w-4xl mx-auto px-4 sm:px-6">
      <div class="text-center mb-10 sm:mb-14">
        <h2 class="text-2xl sm:text-3xl md:text-4xl font-bold text-white mb-3 min-h-[1.2em]">{quickstartText()}</h2>
        <p class="text-[#a0a0b0] max-w-2xl mx-auto text-sm sm:text-base">From zero to a working search index in under a minute.</p>
      </div>

      <div class="space-y-4 sm:space-y-5">
        <!-- Step 1 -->
        <div class="bg-[#13121a] border border-white/5 rounded-xl p-5 sm:p-6">
          <div class="flex items-center gap-3 mb-3">
            <div class="w-7 h-7 rounded-full bg-[#ff4d6d]/20 text-[#ff4d6d] font-bold text-sm flex items-center justify-center">1</div>
            <h3 class="text-white font-semibold">Install</h3>
          </div>
          <p class="text-[#9090a0] text-sm mb-3">macOS &amp; Linux:</p>
          <button onclick={() => copyCmd(installCmd)} class="w-full text-left bg-black/40 border border-white/5 rounded-lg p-3 sm:p-4 font-mono text-xs sm:text-sm text-[#d0d0e0] overflow-x-auto hover:border-white/10 transition-colors flex items-center gap-3 group">
            <span class="text-[#ff4d6d] shrink-0">$</span>
            <span class="flex-1 truncate">curl -sSL https://raw.githubusercontent.com/enekos/marrow/main/install.sh | sh</span>
            {#if copied}
              <Check size={14} class="text-green-400 shrink-0" />
            {:else}
              <Copy size={14} class="text-[#7a7a8a] group-hover:text-white transition-colors shrink-0" />
            {/if}
          </button>
          <p class="text-[#7a7a8a] text-xs mt-2">Windows users: grab the zip from the <a href="https://github.com/enekos/marrow/releases" class="text-[#ff4d6d] hover:underline" target="_blank" rel="noopener">releases page</a>.</p>
        </div>

        <!-- Step 2 -->
        <div class="bg-[#13121a] border border-white/5 rounded-xl p-5 sm:p-6">
          <div class="flex items-center gap-3 mb-3">
            <div class="w-7 h-7 rounded-full bg-[#7c3aed]/20 text-[#7c3aed] font-bold text-sm flex items-center justify-center">2</div>
            <h3 class="text-white font-semibold">Index a folder</h3>
          </div>
          <p class="text-[#9090a0] text-sm mb-3">Point Marrow at your Markdown directory. It writes a single <code>marrow.db</code> file:</p>
          <button onclick={() => copyCmd('marrow sync -dir ./docs -db marrow.db')} class="w-full text-left bg-black/40 border border-white/5 rounded-lg p-3 sm:p-4 font-mono text-xs sm:text-sm text-[#d0d0e0] overflow-x-auto hover:border-white/10 transition-colors flex items-center gap-3 group">
            <span class="text-[#ff4d6d] shrink-0">$</span>
            <span class="flex-1 truncate">marrow sync -dir ./docs -db marrow.db</span>
            <Copy size={14} class="text-[#7a7a8a] group-hover:text-white transition-colors shrink-0" />
          </button>
        </div>

        <!-- Step 3 -->
        <div class="bg-[#13121a] border border-white/5 rounded-xl p-5 sm:p-6">
          <div class="flex items-center gap-3 mb-3">
            <div class="w-7 h-7 rounded-full bg-[#ff4d6d]/20 text-[#ff4d6d] font-bold text-sm flex items-center justify-center">3</div>
            <h3 class="text-white font-semibold">Search</h3>
          </div>
          <p class="text-[#9090a0] text-sm mb-3">Start the server, then query it from anywhere:</p>
          <button onclick={() => copyCmd('marrow serve -db marrow.db -addr :8080')} class="w-full text-left bg-black/40 border border-white/5 rounded-lg p-3 sm:p-4 font-mono text-xs sm:text-sm text-[#d0d0e0] overflow-x-auto hover:border-white/10 transition-colors flex items-center gap-3 group mb-3">
            <span class="text-[#ff4d6d] shrink-0">$</span>
            <span class="flex-1 truncate">marrow serve -db marrow.db -addr :8080</span>
            <Copy size={14} class="text-[#7a7a8a] group-hover:text-white transition-colors shrink-0" />
          </button>
          <button onclick={() => copyCmd('curl -X POST localhost:8080/search -d \'{"q":"how to log in","limit":5}\'')} class="w-full text-left bg-black/40 border border-white/5 rounded-lg p-3 sm:p-4 font-mono text-[11px] sm:text-xs text-[#d0d0e0] overflow-x-auto hover:border-white/10 transition-colors flex items-center gap-3 group">
            <span class="text-[#ff4d6d] shrink-0">$</span>
            <span class="flex-1 truncate">curl -X POST localhost:8080/search -d '{'{'}"q":"how to log in","limit":5{'}'}'</span>
            <Copy size={14} class="text-[#7a7a8a] group-hover:text-white transition-colors shrink-0" />
          </button>
        </div>
      </div>

      <div class="mt-6 text-center text-sm text-[#7a7a8a]">
        Need more? Read the full <a href="#docs" class="text-[#ff4d6d] hover:underline">documentation</a> below or check the <a href="https://github.com/enekos/marrow#readme" class="text-[#ff4d6d] hover:underline" target="_blank" rel="noopener">README on GitHub</a>.
      </div>
    </div>
  </section>

  <!-- Deep documentation -->
  <section id="docs" class="relative z-10 py-16 sm:py-24">
    <div class="max-w-4xl mx-auto px-4 sm:px-6">
      <div class="text-center mb-10 sm:mb-12">
        <h2 class="text-2xl sm:text-3xl md:text-4xl font-bold text-white mb-3 min-h-[1.2em]">{docsText()}</h2>
        <p class="text-[#a0a0b0] text-sm sm:text-base">Under-the-hood detail for the curious.</p>
      </div>

      <!-- Architecture -->
      <div class="border border-white/5 rounded-xl overflow-hidden mb-3 sm:mb-4 bg-[#13121a]">
        <button onclick={() => toggleSection('architecture')} class="w-full flex items-center justify-between px-5 sm:px-6 py-4 sm:py-5 hover:bg-white/5 transition-colors text-left">
          <div class="flex items-center gap-3 text-white font-semibold text-sm sm:text-base">
            <TerminalSquare size={18} class="text-[#ff4d6d] shrink-0" /> Architecture
          </div>
          <ChevronDown size={18} class="text-[#7a7a8a] transition-transform shrink-0 {openSection === 'architecture' ? 'rotate-180' : ''}" />
        </button>
        {#if openSection === 'architecture'}
          <div class="px-5 sm:px-6 pb-5 sm:pb-6 text-[#b0b0c0] text-sm leading-relaxed space-y-4">
            <p>Marrow stores everything in a single SQLite database with three core tables:</p>
            <ul class="list-disc pl-5 space-y-1">
              <li><strong class="text-white">documents</strong> — metadata (path, hash, title, lang, source, doc_type).</li>
              <li><strong class="text-white">documents_fts</strong> — a virtual FTS5 table for full-text search using the <code>unicode61</code> tokenizer.</li>
              <li><strong class="text-white">documents_vec</strong> — a virtual sqlite-vec table storing 384-dimensional float vectors.</li>
            </ul>
            <p>When you sync, Marrow parses Markdown with <code>goldmark</code>, extracts YAML frontmatter (title and lang), strips code blocks to reduce noise, and generates a SHA-256 content hash. Only files with new hashes are re-embedded and re-indexed.</p>
            <p>At search time, Marrow runs two independent queries — one against FTS5 (BM25 scoring) and one against sqlite-vec (cosine distance) — then merges the results with <strong class="text-white">Reciprocal Rank Fusion</strong> (RRF) using a 70/30 blend. Additional heuristics boost title matches, exact phrase matches, and recent documents.</p>
          </div>
        {/if}
      </div>

      <!-- Search Logic -->
      <div class="border border-white/5 rounded-xl overflow-hidden mb-3 sm:mb-4 bg-[#13121a]">
        <button onclick={() => toggleSection('searchlogic')} class="w-full flex items-center justify-between px-5 sm:px-6 py-4 sm:py-5 hover:bg-white/5 transition-colors text-left">
          <div class="flex items-center gap-3 text-white font-semibold text-sm sm:text-base">
            <Search size={18} class="text-[#7c3aed] shrink-0" /> Ranking &amp; search logic
          </div>
          <ChevronDown size={18} class="text-[#7a7a8a] transition-transform shrink-0 {openSection === 'searchlogic' ? 'rotate-180' : ''}" />
        </button>
        {#if openSection === 'searchlogic'}
          <div class="px-5 sm:px-6 pb-5 sm:pb-6 text-[#b0b0c0] text-sm leading-relaxed space-y-4">
            <p>Search is not a simple keyword lookup. Marrow applies several layers of ranking:</p>
            <ol class="list-decimal pl-5 space-y-2">
              <li><strong class="text-white">Stemming &amp; language detection</strong> — Queries are stemmed using pure-Go Snowball stemmers (English Porter2, Spanish, Basque). If no language hint is provided, Marrow auto-detects it from character patterns and stopword frequencies.</li>
              <li><strong class="text-white">Dual retrieval</strong> — FTS5 returns BM25-ranked results; sqlite-vec returns cosine-similarity-ranked results. Both are limited to <code>limit × 3</code> candidates.</li>
              <li><strong class="text-white">RRF fusion</strong> — Each candidate gets a fused score: <code>70% FTS + 30% vector</code>, blending normalized rank positions and raw similarity scores.</li>
              <li><strong class="text-white">Title boost</strong> — If stemmed query tokens appear in the document title, the score is multiplied by up to <code>1.25×</code>.</li>
              <li><strong class="text-white">Exact phrase boost</strong> — Quoted phrases and title substring matches each give an extra <code>1.10×</code> multiplier.</li>
              <li><strong class="text-white">Recency boost</strong> — Documents modified within the last 180 days receive up to <code>1.05×</code>.</li>
            </ol>
            <p>You can filter results by <code>source</code>, <code>doc_type</code> (markdown, issue, pull_request), or <code>lang</code> via the API or the UI below.</p>
          </div>
        {/if}
      </div>

      <!-- Sync -->
      <div class="border border-white/5 rounded-xl overflow-hidden mb-3 sm:mb-4 bg-[#13121a]">
        <button onclick={() => toggleSection('sync')} class="w-full flex items-center justify-between px-5 sm:px-6 py-4 sm:py-5 hover:bg-white/5 transition-colors text-left">
          <div class="flex items-center gap-3 text-white font-semibold text-sm sm:text-base">
            <RefreshCw size={18} class="text-[#ff4d6d] shrink-0" /> Sync sources
          </div>
          <ChevronDown size={18} class="text-[#7a7a8a] transition-transform shrink-0 {openSection === 'sync' ? 'rotate-180' : ''}" />
        </button>
        {#if openSection === 'sync'}
          <div class="px-5 sm:px-6 pb-5 sm:pb-6 text-[#b0b0c0] text-sm leading-relaxed space-y-4">
            <p>Marrow supports three source types, each with incremental sync behavior:</p>
            <ul class="list-disc pl-5 space-y-2">
              <li><strong class="text-white">Local directories</strong> — The watcher crawls the tree and uses <code>mtime</code> as a fast-path filter. Otherwise, the file is read and its SHA-256 hash is compared to the stored hash. Deleted files are removed automatically.</li>
              <li><strong class="text-white">Git repositories</strong> — <code>gitpull.Sync</code> clones with <code>--depth 1</code> on first run, then fetches and hard-resets on subsequent runs. Only <code>.md</code> files that changed between the old and new HEAD are re-indexed.</li>
              <li><strong class="text-white">GitHub API (Issues &amp; PRs)</strong> — Fetches open issues and pull requests, including comments. Each item gets a synthetic path like <code>gh:owner/repo/issues/123</code>. Webhooks keep the index live: opened/edited/reopened events re-index; closed events delete.</li>
            </ul>
            <p>Run <code>marrow sync</code> for a one-off sync, or use <code>marrow reindex</code> to force a full re-index across all configured sources.</p>
          </div>
        {/if}
      </div>

      <!-- Embeddings -->
      <div class="border border-white/5 rounded-xl overflow-hidden mb-3 sm:mb-4 bg-[#13121a]">
        <button onclick={() => toggleSection('embeddings')} class="w-full flex items-center justify-between px-5 sm:px-6 py-4 sm:py-5 hover:bg-white/5 transition-colors text-left">
          <div class="flex items-center gap-3 text-white font-semibold text-sm sm:text-base">
            <Code size={18} class="text-[#7c3aed] shrink-0" /> Embeddings
          </div>
          <ChevronDown size={18} class="text-[#7a7a8a] transition-transform shrink-0 {openSection === 'embeddings' ? 'rotate-180' : ''}" />
        </button>
        {#if openSection === 'embeddings'}
          <div class="px-5 sm:px-6 pb-5 sm:pb-6 text-[#b0b0c0] text-sm leading-relaxed space-y-4">
            <p>Marrow abstracts embeddings behind a simple <code>Func</code> interface. Three providers are supported:</p>
            <ul class="list-disc pl-5 space-y-2">
              <li><strong class="text-white">mock</strong> — Deterministic 384-dim vectors derived from SHA-256 (Box-Muller). Zero-config, fully offline, reproducible. Great for CI and tests.</li>
              <li><strong class="text-white">ollama</strong> — Talks to a local Ollama server (default <code>http://localhost:11434</code>, model <code>nomic-embed-text</code>). Fully local, no API keys.</li>
              <li><strong class="text-white">openai</strong> — Any OpenAI-compatible API (default <code>text-embedding-3-small</code>). Requires <code>api_key</code>; optional <code>base_url</code> for proxies or Azure.</li>
            </ul>
            <p>Configure via the config file or env vars: <code>MARROW_EMBEDDING_PROVIDER</code>, <code>MARROW_EMBEDDING_MODEL</code>, <code>MARROW_EMBEDDING_BASE_URL</code>, <code>MARROW_EMBEDDING_API_KEY</code>.</p>
          </div>
        {/if}
      </div>

      <!-- Configuration -->
      <div class="border border-white/5 rounded-xl overflow-hidden mb-3 sm:mb-4 bg-[#13121a]">
        <button onclick={() => toggleSection('config')} class="w-full flex items-center justify-between px-5 sm:px-6 py-4 sm:py-5 hover:bg-white/5 transition-colors text-left">
          <div class="flex items-center gap-3 text-white font-semibold text-sm sm:text-base">
            <Settings size={18} class="text-[#ff4d6d] shrink-0" /> Configuration
          </div>
          <ChevronDown size={18} class="text-[#7a7a8a] transition-transform shrink-0 {openSection === 'config' ? 'rotate-180' : ''}" />
        </button>
        {#if openSection === 'config'}
          <div class="px-5 sm:px-6 pb-5 sm:pb-6 text-[#b0b0c0] text-sm leading-relaxed space-y-4">
            <p>Marrow uses a four-layer configuration cascade (later layers override earlier ones):</p>
            <ol class="list-decimal pl-5 space-y-1">
              <li>Hardcoded defaults</li>
              <li>User config: <code class="text-white">~/.config/marrow/config.toml</code></li>
              <li>Project config: <code class="text-white">.marrow.toml</code> (discovered by walking up from the working directory)</li>
              <li>Environment variables with the <code class="text-white">MARROW_</code> prefix</li>
            </ol>
            <p>Example <code>.marrow.toml</code>:</p>
            <pre class="bg-black/40 border border-white/5 rounded-lg p-3 sm:p-4 font-mono text-[11px] sm:text-xs text-[#a0a0b0] overflow-x-auto">[server]
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

      <!-- CLI Commands -->
      <div class="border border-white/5 rounded-xl overflow-hidden mb-3 sm:mb-4 bg-[#13121a]">
        <button onclick={() => toggleSection('cli')} class="w-full flex items-center justify-between px-5 sm:px-6 py-4 sm:py-5 hover:bg-white/5 transition-colors text-left">
          <div class="flex items-center gap-3 text-white font-semibold text-sm sm:text-base">
            <Terminal size={18} class="text-[#7c3aed] shrink-0" /> CLI commands
          </div>
          <ChevronDown size={18} class="text-[#7a7a8a] transition-transform shrink-0 {openSection === 'cli' ? 'rotate-180' : ''}" />
        </button>
        {#if openSection === 'cli'}
          <div class="px-5 sm:px-6 pb-5 sm:pb-6 text-[#b0b0c0] text-sm leading-relaxed space-y-4">
            <ul class="space-y-3">
              <li>
                <code class="text-white bg-white/10 px-1.5 py-0.5 rounded">marrow sync</code>
                <p class="mt-1">One-off incremental sync of a local directory. Flags: <code>-dir</code>, <code>-db</code>, <code>-source</code>, <code>-default-lang</code>. Use <code>-all</code> to sync every configured source.</p>
              </li>
              <li>
                <code class="text-white bg-white/10 px-1.5 py-0.5 rounded">marrow serve</code>
                <p class="mt-1">Starts the HTTP API and serves this landing page locally. Flags include <code>-db</code>, <code>-addr</code>, <code>-detect-lang</code>, <code>-default-lang</code>, plus GitHub options.</p>
              </li>
              <li>
                <code class="text-white bg-white/10 px-1.5 py-0.5 rounded">marrow status</code>
                <p class="mt-1">Prints database stats: total docs, size, last sync, breakdown by source and doc type.</p>
              </li>
              <li>
                <code class="text-white bg-white/10 px-1.5 py-0.5 rounded">marrow reindex</code>
                <p class="mt-1">Force a full reindex of all configured sources.</p>
              </li>
              <li>
                <code class="text-white bg-white/10 px-1.5 py-0.5 rounded">marrow maintain</code>
                <p class="mt-1">Runs VACUUM and prunes orphaned FTS/vector rows. Use <code>-backup</code> to create a timestamped backup first.</p>
              </li>
            </ul>
          </div>
        {/if}
      </div>

      <!-- API -->
      <div class="border border-white/5 rounded-xl overflow-hidden mb-3 sm:mb-4 bg-[#13121a]">
        <button onclick={() => toggleSection('api')} class="w-full flex items-center justify-between px-5 sm:px-6 py-4 sm:py-5 hover:bg-white/5 transition-colors text-left">
          <div class="flex items-center gap-3 text-white font-semibold text-sm sm:text-base">
            <Code size={18} class="text-[#ff4d6d] shrink-0" /> API endpoints
          </div>
          <ChevronDown size={18} class="text-[#7a7a8a] transition-transform shrink-0 {openSection === 'api' ? 'rotate-180' : ''}" />
        </button>
        {#if openSection === 'api'}
          <div class="px-5 sm:px-6 pb-5 sm:pb-6 text-[#b0b0c0] text-sm leading-relaxed space-y-4">
            <div class="space-y-3">
              <div>
                <div class="font-mono text-xs text-[#ff4d6d] mb-1">POST /search</div>
                <pre class="bg-black/40 border border-white/5 rounded-lg p-3 font-mono text-[11px] sm:text-xs text-[#a0a0b0] overflow-x-auto">{'{'} "q": "authentication flow", "limit": 10, "lang": "en", "source": "docs", "doc_type": "markdown" {'}'}</pre>
                <p class="mt-1">Returns ranked results with <code>id</code>, <code>path</code>, <code>title</code>, <code>doc_type</code>, and <code>score</code>.</p>
              </div>
              <div>
                <div class="font-mono text-xs text-[#ff4d6d] mb-1">GET /stats</div>
                <p>Returns aggregate stats: <code>TotalDocs</code>, <code>DBSizeBytes</code>, <code>Sources</code>, <code>BySource</code>, <code>ByDocType</code>, <code>LastSyncAt</code>.</p>
              </div>
              <div>
                <div class="font-mono text-xs text-[#ff4d6d] mb-1">POST /webhook</div>
                <p>Accepts a legacy re-sync trigger (<code>X-Marrow-Secret</code>) <em>or</em> real GitHub App webhooks signed with <code>X-Hub-Signature-256</code>.</p>
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
  <section id="search" class="relative z-10 py-16 sm:py-24 border-y border-white/5 bg-[#0f0e14]">
    <div class="max-w-3xl mx-auto px-4 sm:px-6">
      <div class="text-center mb-8 sm:mb-10">
        <h2 class="text-2xl sm:text-3xl md:text-4xl font-bold text-white mb-3 min-h-[1.2em]">{searchText()}</h2>
        <p class="text-[#a0a0b0] text-sm sm:text-base">A live query against the Marrow instance serving this page.</p>
      </div>

      {#if searchAvailable === false}
        <div class="bg-[#13121a] border border-white/5 rounded-xl p-6 sm:p-8 text-center">
          <p class="text-[#a0a0b0] text-sm leading-relaxed mb-4">
            This page is hosted statically — no live Marrow server is attached. To try the live demo, install Marrow and open the landing page from your own instance:
          </p>
          <button onclick={() => copyCmd('marrow serve -db marrow.db -addr :8080')} class="inline-flex items-center gap-2 px-4 py-2 bg-black/40 border border-white/5 rounded-lg font-mono text-xs sm:text-sm text-[#d0d0e0] hover:border-white/10 transition-colors">
            <span class="text-[#ff4d6d]">$</span>
            marrow serve -db marrow.db -addr :8080
            {#if copied}<Check size={14} class="text-green-400" />{:else}<Copy size={14} />{/if}
          </button>
          <p class="text-[#7a7a8a] text-xs mt-3">Then visit <code>http://localhost:8080</code>.</p>
        </div>
      {:else}
        <div class="bg-[#13121a] border border-white/5 rounded-xl p-5 sm:p-6 shadow-xl">
          <div class="flex flex-col md:flex-row gap-3 mb-4">
            <input
              type="text"
              bind:value={query}
              onkeydown={handleKey}
              placeholder="Search..."
              class="flex-1 px-4 py-3 bg-black/30 border border-white/10 rounded-lg text-white placeholder-[#606070] focus:outline-none focus:border-[#ff4d6d]/50 text-sm sm:text-base"
            />
            <select
              bind:value={sourceFilter}
              class="px-4 py-3 bg-black/30 border border-white/10 rounded-lg text-white focus:outline-none focus:border-[#ff4d6d]/50 text-sm sm:text-base"
            >
              <option value="">All sources</option>
              {#each sources as src}
                <option value={src}>{src}</option>
              {/each}
            </select>
            <select
              bind:value={docTypeFilter}
              class="px-4 py-3 bg-black/30 border border-white/10 rounded-lg text-white focus:outline-none focus:border-[#ff4d6d]/50 text-sm sm:text-base"
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
                  <h3 class="text-white font-semibold mb-1 break-words">{res.title || res.path}</h3>
                  <div class="text-xs text-[#808090] break-all">
                    {res.doc_type} &middot; {res.path} &middot; score {typeof res.score === 'number' ? res.score.toFixed(3) : res.score}
                  </div>
                </div>
              {/each}
            </div>
          {:else if !searching && query && !searchError}
            <div class="mt-6 text-[#707080] text-sm">No results found.</div>
          {/if}
        </div>
      {/if}
    </div>
  </section>

  <!-- CTA -->
  <section class="relative z-10 py-16 sm:py-20">
    <div class="max-w-3xl mx-auto px-4 sm:px-6 text-center">
      <h2 class="text-2xl sm:text-3xl md:text-4xl font-bold text-white mb-4">Ready to try it?</h2>
      <p class="text-[#a0a0b0] mb-6 sm:mb-8 text-sm sm:text-base max-w-xl mx-auto">
        One binary, one command, one SQLite file. No accounts, no telemetry, no cloud.
      </p>
      <div class="flex flex-col sm:flex-row gap-3 sm:gap-4 justify-center">
        <a href="#quickstart" class="px-6 py-3 bg-[#ff4d6d] text-white rounded-full font-semibold text-sm hover:shadow-[0_0_25px_rgba(255,77,109,0.4)] hover:-translate-y-0.5 transition-all flex items-center justify-center gap-2">
          Get started <ArrowRight size={16} />
        </a>
        <a href="https://github.com/enekos/marrow" target="_blank" rel="noopener" class="px-6 py-3 border border-white/10 rounded-full text-white text-sm hover:bg-white/10 hover:border-[#ff4d6d]/50 transition-all flex items-center justify-center gap-2">
          <GitBranch size={14} /> Star on GitHub
        </a>
      </div>
    </div>
  </section>

  <!-- Footer -->
  <footer class="relative z-10 py-8 sm:py-10 bg-[#0b0a10] border-t border-white/5">
    <div class="max-w-6xl mx-auto px-4 sm:px-6 flex flex-col sm:flex-row justify-between items-center gap-4 text-[#707080] text-xs sm:text-sm">
      <div class="flex items-center gap-2 font-semibold text-white">
        <Database size={18} class="text-[#ff4d6d]" /> Marrow
      </div>
      <div class="flex items-center gap-4 sm:gap-6">
        <a href="https://github.com/enekos/marrow" target="_blank" rel="noopener" class="hover:text-white transition-colors">GitHub</a>
        <a href="https://github.com/enekos/marrow/releases" target="_blank" rel="noopener" class="hover:text-white transition-colors">Releases</a>
        <a href="https://github.com/enekos/marrow/issues" target="_blank" rel="noopener" class="hover:text-white transition-colors">Issues</a>
      </div>
      <div>
        MIT · Built by <a href="https://github.com/enekos" target="_blank" rel="noopener" class="text-[#a0a0b0] hover:text-white transition-colors">enekos</a>
      </div>
    </div>
  </footer>
</div>
