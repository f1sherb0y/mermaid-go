// Layout test inputs. A case is { name, opts, multigraph, compound, nodes,
// edges, mayThrow? }: nodes are declared in order (setNode then setParent),
// then edges (see ref.mjs build). All generators are seeded.

export function mulberry(seed) {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

function tools(rng) {
  const int = (lo, hi) => lo + Math.floor(rng() * (hi - lo + 1));
  const pick = (a) => a[Math.floor(rng() * a.length)];
  const shuffle = (a) => {
    for (let i = a.length - 1; i > 0; i--) {
      const j = Math.floor(rng() * (i + 1));
      [a[i], a[j]] = [a[j], a[i]];
    }
    return a;
  };
  // Text-like sizes: whole pixels, 1/64 fractions, now and then zero.
  const size = (lo, hi) => {
    const r = rng();
    if (r < 0.05) return 0;
    if (r < 0.6) return int(lo, hi);
    return int(lo * 64, hi * 64) / 64;
  };
  return { int, pick, shuffle, size };
}

function randOpts(rng, t) {
  const o = {};
  const rd = t.pick([undefined, 'TB', 'BT', 'LR', 'RL', 'TB', 'LR', 'TB', 'tb', 'lr', 'bt', 'rl']);
  if (rd) o.rankdir = rd;
  if (rng() < 0.7) o.nodesep = t.pick([50, 20, 0, 35.5, 100]);
  if (rng() < 0.5) o.edgesep = t.pick([20, 10, 0, 7.25]);
  if (rng() < 0.7) o.ranksep = t.pick([50, 20, 75, 33.3]);
  if (rng() < 0.5) o.marginx = t.pick([8, 0, 20]);
  if (rng() < 0.5) o.marginy = t.pick([8, 0, 13.5]);
  if (rng() < 0.3) o.ranker = t.pick(['network-simplex', 'tight-tree', 'longest-path']);
  if (rng() < 0.15) o.acyclicer = 'greedy';
  if (rng() < 0.08) o.align = t.pick(['UL', 'UR', 'DL', 'DR', 'dr']);
  return o;
}

function randEdge(rng, t, v, w, o, { labelP, name, minlen0 }) {
  const e = { v, w };
  if (name !== undefined) e.name = name;
  if (rng() < 0.85) e.minlen = rng() < 0.85 ? 1 : t.pick(minlen0 ? [0, 2, 3] : [2, 3]);
  if (rng() < 0.85) e.weight = rng() < 0.8 ? 1 : t.pick([0, 2, 3, 5, o.acyclicer ? 1 : 0.5]);
  if (rng() < labelP) {
    e.width = t.size(10, 120);
    e.height = t.size(10, 40);
  } else if (rng() < 0.4) {
    e.width = 0;
    e.height = 0;
  }
  if (rng() < 0.6) e.labelpos = t.pick(['c', 'c', 'l', 'r', 'L', 'C', 'R']);
  if (rng() < 0.3) e.labeloffset = t.pick([10, 0, 5, 12.5]);
  return e;
}

// Random edges among ids; acyclic ones follow a random order of the ids.
function randEdges(rng, t, ids, m, o, { cyclic, multigraph, selfLoops, labelP, minlen0 }) {
  const order = t.shuffle([...ids]);
  const rank = new Map(order.map((v, i) => [v, i]));
  const edges = [];
  for (let k = 0; k < m && ids.length > 1; k++) {
    let v = t.pick(ids), w = t.pick(ids);
    if (v === w && !(selfLoops && rng() < 0.5)) continue;
    if (!cyclic && rank.get(v) > rank.get(w)) [v, w] = [w, v];
    const name = multigraph && rng() < 0.6 ? `e${k}` : undefined;
    edges.push(randEdge(rng, t, v, w, o, { labelP, name, minlen0: minlen0 && !cyclic && v !== w }));
  }
  return edges;
}

function nodeSize(t) {
  return { width: t.size(10, 180), height: t.size(10, 80) };
}

export function flat() {
  const out = [];
  for (let c = 0; c < 260; c++) {
    const rng = mulberry(1000 + c);
    const t = tools(rng);
    const r = rng();
    const n = r < 0.4 ? t.int(2, 12) : r < 0.85 ? t.int(13, 40) : t.int(41, 110);
    const m = Math.round(n * (0.5 + rng() * 1.7));
    const o = randOpts(rng, t);
    const cyclic = rng() < 0.5;
    const multigraph = rng() < 0.5;
    const idStyle = t.pick(['n', 'int', 'mixed']);
    const ids = [];
    for (let i = 0; i < n; i++) {
      ids.push(idStyle === 'n' ? `n${i}` : idStyle === 'int' ? String(i) : rng() < 0.5 ? String(i * 3) : `v${i}`);
    }
    const nodes = t.shuffle([...ids]).map((id) => (rng() < 0.03 ? { id, nolabel: true } : { id, ...nodeSize(t) }));
    const edges = randEdges(rng, t, ids, m, o, {
      cyclic, multigraph, selfLoops: rng() < 0.2, labelP: t.pick([0, 0.2, 0.5, 1]), minlen0: rng() < 0.1,
    });
    // Now and then an edge to a node never declared.
    if (rng() < 0.05) edges.push({ v: ids[0], w: 'implicit' });
    out.push({ name: `flat-${c}`, opts: o, multigraph, compound: rng() < 0.3, nodes, edges });
  }
  return out;
}

// compound makes cluster trees 1 to 8 deep with nodes at every depth, edges
// across depths, empty clusters (nodes nothing is put in), and now and then
// an edge on a cluster, which dagre cannot lay out.
export function compound() {
  const out = [];
  for (let c = 0; c < 300; c++) {
    const rng = mulberry(5000 + c);
    const t = tools(rng);
    const depth = 1 + (c % 8);
    const nClusters = depth + t.int(0, c % 3 === 0 ? 12 : 5);
    const r = rng();
    const nLeaves = r < 0.4 ? t.int(1, 10) : r < 0.85 ? t.int(10, 35) : t.int(35, 90);
    const o = randOpts(rng, t);
    const ids = t.pick(['n', 'int']);
    const cid = (i) => (ids === 'n' ? `c${i}` : String(100 + i));
    const lid = (i) => (ids === 'n' ? `n${i}` : String(i));

    // Cluster tree: a chain depth deep, then random clusters under any
    // cluster that is not already at the bottom.
    const parent = new Map();
    const level = new Map();
    for (let i = 0; i < nClusters; i++) {
      if (i < depth) {
        if (i > 0) parent.set(cid(i), cid(i - 1));
        level.set(cid(i), i + 1);
        continue;
      }
      const choices = [...level.keys()].filter((k) => level.get(k) < depth);
      const p = rng() < 0.25 || !choices.length ? undefined : t.pick(choices);
      if (p !== undefined) parent.set(cid(i), p);
      level.set(cid(i), p === undefined ? 1 : level.get(p) + 1);
    }
    const clusters = [...level.keys()];
    // Some clusters stay empty: plain nodes that look like clusters.
    const empty = new Set(clusters.filter(() => rng() < 0.12));
    const full = clusters.filter((k) => !empty.has(k));
    const leaves = [];
    for (let i = 0; i < nLeaves; i++) {
      const id = lid(i);
      leaves.push(id);
      // Bias towards deep clusters so every depth gets nodes.
      if (rng() < 0.2 || !full.length) continue;
      const k = rng() < 0.4 ? cid(depth - 1) : t.pick(full);
      if (!empty.has(k)) parent.set(id, k);
    }
    // The deepest cluster of the chain always holds a node.
    if (!empty.has(cid(depth - 1)) && leaves.length) parent.set(leaves[0], cid(depth - 1));

    const all = [...clusters, ...leaves];
    const order = rng() < 0.5 ? t.shuffle([...all]) : rng() < 0.5 ? all : [...all].reverse();
    const nodes = order.map((id) => {
      const n = { id, ...nodeSize(t) };
      if (parent.has(id)) n.parent = parent.get(id);
      return n;
    });
    // Edges among nodes without children (the empty clusters included).
    const hasChildren = new Set(parent.values());
    const ends = all.filter((v) => !hasChildren.has(v));
    const multigraph = rng() < 0.7;
    const edges = randEdges(rng, t, ends, Math.round(ends.length * (0.4 + rng() * 1.6)), o, {
      cyclic: rng() < 0.4, multigraph, selfLoops: rng() < 0.15, labelP: t.pick([0, 0.3, 0.6]), minlen0: false,
    });
    const cs = {
      name: `compound-${c}-d${depth}`, opts: o, multigraph, compound: true, nodes, edges,
    };
    if (c % 25 === 7 && hasChildren.size) {
      const k = t.pick([...hasChildren]);
      edges.push(rng() < 0.5 ? { v: k, w: t.pick(ends) } : { v: t.pick(ends), w: k });
      cs.mayThrow = true;
      cs.name += '-clusteredge';
    }
    out.push(cs);
  }
  return out;
}

// flowchart makes graphs shaped like mermaid flowcharts after mermaid's own
// preparation: text-sized nodes, centred edge labels, decisions, loops,
// nested subgraphs, self-loops replaced by mermaid's two helper nodes.
export function flowchart() {
  const out = [];
  for (let c = 0; c < 160; c++) {
    const rng = mulberry(9000 + c);
    const t = tools(rng);
    const dir = t.pick(['TB', 'TB', 'LR', 'BT', 'RL']);
    const o = { rankdir: dir, nodesep: t.pick([50, 50, 30]), ranksep: t.pick([50, 50, 40]), marginx: 8, marginy: 8 };
    const n = c < 120 ? t.int(3, 30) : t.int(30, 80);
    const ids = [];
    for (let i = 0; i < n; i++) ids.push(rng() < 0.15 ? String(i + 1) : i < 26 ? String.fromCharCode(65 + i) : `N${i}`);
    const uniq = [...new Set(ids)];
    const text = () => ({ width: Math.round(rng() * 120 * 64) / 64 + 32, height: t.pick([54, 54, 78, 38.5]) });
    const parent = new Map();
    const subs = [];
    const nSubs = rng() < 0.5 ? 0 : t.int(1, 6);
    for (let s = 0; s < nSubs; s++) {
      const id = `subGraph${s}`;
      subs.push(id);
      if (s > 0 && rng() < 0.5) parent.set(id, t.pick(subs.slice(0, s)));
    }
    for (const v of uniq) if (subs.length && rng() < 0.5) parent.set(v, t.pick(subs));
    // Subgraphs with no nodes in them are dropped, as mermaid has none.
    const used = new Set(parent.values());
    const nodes = [];
    for (const s of subs) if (used.has(s)) nodes.push({ id: s, width: 0, height: 0, ...(parent.has(s) ? { parent: parent.get(s) } : {}) });
    for (const v of uniq) nodes.push({ id: v, ...text(), ...(parent.has(v) ? { parent: parent.get(v) } : {}) });
    const leaves = uniq.filter((v) => !used.has(v));
    const edges = [];
    let k = 0;
    const edge = (v, w, label) => {
      const e = { v, w, name: `L_${v}_${w}_${k++}`, minlen: rng() < 0.9 ? 1 : 2, weight: 1, labelpos: 'c', labeloffset: 10 };
      if (label) {
        e.width = Math.round((20 + rng() * 80) * 64) / 64;
        e.height = 24;
      } else {
        e.width = 0;
        e.height = 0;
      }
      edges.push(e);
    };
    // A main path with decisions branching off, merging back and looping.
    for (let i = 1; i < leaves.length; i++) {
      const from = rng() < 0.7 ? leaves[i - 1] : t.pick(leaves.slice(0, i));
      edge(from, leaves[i], rng() < 0.3);
      if (rng() < 0.25) edge(from, t.pick(leaves), true);
    }
    for (const v of leaves) {
      if (rng() < 0.06) {
        // mermaid's self-loop: v -> v---v---1 -> v---v---2 -> v.
        const a = `${v}---${v}---1`, b = `${v}---${v}---2`;
        const p = parent.get(v);
        nodes.push({ id: a, width: 10, height: 10, ...(p ? { parent: p } : {}) });
        nodes.push({ id: b, width: 10, height: 10, ...(p ? { parent: p } : {}) });
        const base = { minlen: 1, weight: 1, labelpos: 'c', labeloffset: 10, width: 0, height: 0 };
        edges.push({ ...base, v, w: a, name: `${v}-cyclic-special-0` });
        edges.push({ ...base, v: a, w: b, name: `${v}-cyclic-special-1` });
        edges.push({ ...base, v: b, w: v, name: `${v}-cyclic-special-2` });
      }
    }
    out.push({ name: `flowchart-${c}-${dir}`, opts: o, multigraph: true, compound: true, nodes, edges });
  }
  return out;
}

// idCases use node ids whose JS property order differs from insertion
// order (array indices) or whose JS and Go string order differ.
export function idCases() {
  const pool = ['0', '1', '2', '10', '9', '007', '-1', '1.5', '4294967294', '4294967295', 'length',
    'b', 'a', 'B', '\u{1F600}', '\u{1F601}x', '！', 'z', 'é', 'z', '_d1', '_root3', 'x y'];
  const out = [];
  for (let c = 0; c < 40; c++) {
    const rng = mulberry(20000 + c);
    const t = tools(rng);
    const ids = t.shuffle([...pool]).slice(0, t.int(4, pool.length - 1));
    const o = randOpts(rng, t);
    const multigraph = rng() < 0.5;
    const compound = c % 3 === 0;
    const nodes = ids.map((id) => ({ id, ...nodeSize(t) }));
    if (compound) {
      // A couple of clusters with array-index names.
      nodes.push({ id: '20', width: 0, height: 0 }, { id: 'K', width: 0, height: 0, parent: '20' });
      for (const n of nodes.slice(0, ids.length)) if (rng() < 0.4) n.parent = rng() < 0.5 ? '20' : 'K';
    }
    const ends = nodes.filter((n) => n.id !== '20' && n.id !== 'K').map((n) => n.id);
    const edges = randEdges(rng, t, ends, ends.length * 2, o, {
      cyclic: rng() < 0.5, multigraph, selfLoops: rng() < 0.3, labelP: 0.4, minlen0: false,
    });
    out.push({ name: `ids-${c}`, opts: o, multigraph, compound, nodes, edges });
  }
  return out;
}

export function edgeCases() {
  const n = (id, extra = {}) => ({ id, width: 40, height: 20, ...extra });
  const cases = [
    { name: 'empty', nodes: [], edges: [] },
    { name: 'one-node', nodes: [n('a')], edges: [] },
    { name: 'self-loop', nodes: [n('a')], edges: [{ v: 'a', w: 'a', width: 20, height: 10 }] },
    { name: 'two-loose', nodes: [n('a'), n('b')], edges: [] },
    { name: 'zero-size', nodes: [n('a', { width: 0, height: 0 }), n('b', { width: 0, height: 0 })], edges: [{ v: 'a', w: 'b' }] },
    { name: 'parallel', multigraph: true, nodes: [n('a'), n('b')], edges: [
      { v: 'a', w: 'b', name: 'x', width: 30, height: 10 }, { v: 'a', w: 'b', name: 'y', width: 30, height: 10 }, { v: 'b', w: 'a', name: 'z' }] },
    { name: 'long-label-chain', nodes: ['a', 'b', 'c'].map((v) => n(v)), edges: [
      { v: 'a', w: 'c', minlen: 5, width: 50, height: 20, labelpos: 'l' }, { v: 'a', w: 'b' }, { v: 'b', w: 'c' }] },
    { name: 'minlen0', nodes: [n('a'), n('b'), n('c')], edges: [{ v: 'a', w: 'b', minlen: 0 }, { v: 'a', w: 'c' }] },
    { name: 'weights0', nodes: ['a', 'b', 'c', 'd'].map((v) => n(v)), edges: [
      { v: 'a', w: 'c', weight: 0 }, { v: 'b', w: 'c', weight: 0 }, { v: 'a', w: 'd', weight: 0 }, { v: 'b', w: 'd', weight: 0 }] },
    { name: 'nolabel-nodes', nodes: [{ id: 'a', nolabel: true }, n('b')], edges: [{ v: 'a', w: 'b' }, { v: 'b', w: 'ghost' }] },
    { name: 'cluster-one-child', compound: true, nodes: [n('c'), n('a', { parent: 'c' })], edges: [] },
    { name: 'cluster-before-parent', compound: true, nodes: [n('a', { parent: 'c' }), n('b', { parent: 'c' }), n('c')], edges: [{ v: 'a', w: 'b' }] },
    { name: 'cluster-parent-undeclared', compound: true, nodes: [n('a', { parent: 'c' }), n('b')], edges: [{ v: 'a', w: 'b' }] },
    { name: 'cluster-chain-8', compound: true, nodes: [
      ...[1, 2, 3, 4, 5, 6, 7, 8].map((i) => n(`c${i}`, i > 1 ? { parent: `c${i - 1}` } : {})), n('x', { parent: 'c8' }), n('y')],
      edges: [{ v: 'y', w: 'x' }, { v: 'x', w: 'y', width: 30, height: 12 }] },
    { name: 'cluster-edge', compound: true, mayThrow: true, nodes: [n('c'), n('a', { parent: 'c' }), n('b')], edges: [{ v: 'c', w: 'b' }] },
    { name: 'cluster-self-loop', compound: true, mayThrow: true, nodes: [n('c'), n('a', { parent: 'c' })], edges: [{ v: 'c', w: 'c' }] },
    { name: 'empty-cluster', compound: true, nodes: [n('c', { width: 0, height: 0 }), n('a'), n('d'), n('b', { parent: 'd' })], edges: [{ v: 'a', w: 'c' }, { v: 'c', w: 'b' }] },
  ];
  const dirs = ['TB', 'LR', 'BT', 'RL'];
  const out = [];
  for (const c of cases) {
    for (const rankdir of dirs) {
      out.push({ multigraph: false, compound: false, ...c, name: `${c.name}-${rankdir}`, opts: { rankdir } });
    }
  }
  return out;
}

// benchSpecs are the benchmark graphs: flat and compound, mermaid-like
// attributes, 200 to 500 nodes. [name, seed, nodes, edges, clusters,
// depth]; the seeds are the first that dagre lays out without hitting its
// ordering bug (most random compound graphs this size do hit it).
export const benchSpecs = [
  ['flat-200x400', 1, 200, 400, 0, 0],
  ['compound-200x400', 1, 200, 400, 25, 4],
  ['compound-500x900', 58, 500, 900, 50, 5],
];

export function benchCase(name, seed, n, m, nClusters, depth) {
  {
    const rng = mulberry(seed);
    const t = tools(rng);
    const o = { rankdir: 'TB', nodesep: 50, ranksep: 50, marginx: 8, marginy: 8 };
    const parent = new Map();
    const clusters = [];
    for (let i = 0; i < nClusters; i++) {
      const id = `cluster${i}`;
      const lvl = (k) => { let d = 1; for (let p = parent.get(k); p; p = parent.get(p)) d++; return d; };
      const choices = clusters.filter((k) => lvl(k) < depth);
      if (choices.length && rng() < 0.6) parent.set(id, t.pick(choices));
      clusters.push(id);
    }
    const leaves = [];
    for (let i = 0; i < n; i++) {
      leaves.push(`n${i}`);
      if (clusters.length && rng() < 0.7) parent.set(`n${i}`, t.pick(clusters));
    }
    const used = new Set(parent.values());
    const nodes = [];
    for (const k of clusters) if (used.has(k)) nodes.push({ id: k, width: 0, height: 0, ...(parent.has(k) ? { parent: parent.get(k) } : {}) });
    for (const v of leaves) nodes.push({ id: v, width: Math.round(rng() * 100 * 64) / 64 + 40, height: 54, ...(parent.has(v) ? { parent: parent.get(v) } : {}) });
    const ends = leaves;
    const edges = [];
    const order = new Map(ends.map((v, i) => [v, i]));
    for (let k = 0; edges.length < m; k++) {
      let v = t.pick(ends), w = t.pick(ends);
      if (v === w) continue;
      // Mostly forward, a few back edges.
      if (order.get(v) > order.get(w) && rng() < 0.9) [v, w] = [w, v];
      // Prefer nearby nodes, as in real charts.
      if (Math.abs(order.get(v) - order.get(w)) > 30 && rng() < 0.7) continue;
      const label = rng() < 0.3;
      edges.push({ v, w, name: `L${k}`, minlen: 1, weight: 1, labelpos: 'c', labeloffset: 10,
        width: label ? Math.round((20 + rng() * 60) * 64) / 64 : 0, height: label ? 24 : 0 });
    }
    return { name, opts: o, multigraph: true, compound: true, nodes, edges };
  }
}

// nested makes mermaid-shaped charts whose subgraphs nest 1 to 8 deep, as
// trees (several subgraphs per level) or as one long spine, with nodes at
// every level, edges between any two depths and between sibling subtrees,
// empty subgraphs, and now and then an edge on a subgraph (dagre throws).
export function nested() {
  const out = [];
  for (let c = 0; c < 160; c++) {
    const rng = mulberry(30000 + c);
    const t = tools(rng);
    const depth = 1 + (c % 8);
    const spine = c % 3 === 0;
    const dir = t.pick(['TB', 'LR', 'BT', 'RL']);
    const o = { rankdir: dir, nodesep: 50, ranksep: 50, marginx: 8, marginy: 8 };
    if (rng() < 0.3) o.edgesep = t.pick([10, 20]);
    const parent = new Map();
    const level = new Map();
    const subs = [];
    let s = 0;
    const grow = (p, d) => {
      const kids = spine ? 1 : t.int(1, d <= 2 ? 3 : 2);
      for (let k = 0; k < kids && subs.length < 40; k++) {
        const id = `sg${s++}`;
        subs.push(id);
        level.set(id, d);
        if (p) parent.set(id, p);
        if (d < depth && (spine || rng() < 0.75)) grow(id, d + 1);
      }
    };
    grow(undefined, 1);
    const nodes = [];
    const leaves = [];
    const text = () => ({ width: Math.round((30 + rng() * 120) * 64) / 64, height: t.pick([54, 54, 38]) });
    // Nodes at the top level and in most subgraphs, a few subgraphs left
    // empty (they become plain nodes).
    const empty = new Set(subs.filter((k) => !spine && rng() < 0.1));
    let n = 0;
    for (let i = 0; i < t.int(1, 4); i++) leaves.push(`N${n++}`);
    for (const k of subs) {
      if (empty.has(k)) continue;
      for (let i = 0, m = t.int(spine ? 1 : 0, 3); i < m; i++) {
        const id = rng() < 0.2 ? String(n++) : `N${n++}`;
        leaves.push(id);
        parent.set(id, k);
      }
    }
    const hasKids = new Set(parent.values());
    for (const k of subs) {
      const node = { id: k, width: 0, height: 0 };
      if (empty.has(k) || !hasKids.has(k)) Object.assign(node, text());
      if (parent.has(k)) node.parent = parent.get(k);
      nodes.push(node);
    }
    for (const v of leaves) nodes.push({ id: v, ...text(), ...(parent.has(v) ? { parent: parent.get(v) } : {}) });
    // Declaration order: mermaid lists subgraphs first; sometimes reverse
    // so children come before their parents.
    if (rng() < 0.3) nodes.reverse();
    const ends = [...leaves, ...subs.filter((k) => !hasKids.has(k))];
    const edges = [];
    let k = 0;
    const m = Math.round(ends.length * (0.8 + rng()));
    for (let i = 0; i < m && ends.length > 1; i++) {
      // Half of the edges between neighbours in declaration order (chains
      // within a subgraph), half between any two nodes (across depths).
      const a = t.int(0, ends.length - 1);
      const b = rng() < 0.5 ? Math.min(ends.length - 1, a + 1) : t.int(0, ends.length - 1);
      if (a === b) continue;
      const [v, w] = rng() < 0.85 ? [ends[Math.min(a, b)], ends[Math.max(a, b)]] : [ends[a], ends[b]];
      const label = rng() < 0.3;
      edges.push({ v, w, name: `L_${v}_${w}_${k++}`, minlen: rng() < 0.9 ? 1 : 2, weight: 1, labelpos: 'c', labeloffset: 10,
        width: label ? Math.round((20 + rng() * 60) * 64) / 64 : 0, height: label ? 24 : 0 });
    }
    const cs = { name: `nested-${c}-d${depth}${spine ? '-spine' : ''}-${dir}`, opts: o, multigraph: true, compound: true, nodes, edges };
    if (c % 20 === 13 && hasKids.size) {
      const sg = t.pick([...hasKids]);
      edges.push({ v: t.pick(ends), w: sg, name: 'toCluster' });
      cs.mayThrow = true;
      cs.name += '-clusteredge';
    }
    out.push(cs);
  }
  return out;
}
