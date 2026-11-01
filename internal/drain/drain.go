package drain

import (
	"strings"
	"sync"

	"github.com/OrenVill/Argus/internal/meta"
)

const wildcardToken = "<*>"

// maxClustersPerTree bounds in-memory templates per (env, service) per spec §11 RSS target.
const maxClustersPerTree = 2048

// Cluster is the result of matching a masked log line.
type Cluster struct {
	Template string
}

// Forest holds Drain trees keyed by (env, service).
//
// Contract (Track A): use NewForest for shared multi-service forests, or NewProcessForest
// when one process serves a single meta pair. Match requires meta when using Forest directly;
// ProcessForest.Match(line) scopes metadata at construction time.
type Forest struct {
	depth     int
	threshold float64
	mu        sync.RWMutex
	trees     map[string]*tree
}

// NewForest creates a forest with the given depth and similarity threshold.
// simThreshold must be positive (enforce via config.Validate before calling).
func NewForest(depth int, threshold float64) *Forest {
	if depth <= 0 {
		depth = 4
	}
	return &Forest{
		depth:     depth,
		threshold: threshold,
		trees:     make(map[string]*tree),
	}
}

// ProcessForest is a Drain forest scoped to one (env, service) pair (design-spec API).
type ProcessForest struct {
	inner *Forest
	m     meta.Meta
}

// NewProcessForest creates a forest for a single metadata pair (design spec: NewForest(meta, depth, simThreshold)).
func NewProcessForest(m meta.Meta, depth int, simThreshold float64) *ProcessForest {
	return &ProcessForest{
		inner: NewForest(depth, simThreshold),
		m:     m,
	}
}

// Match parses a masked line into a template cluster (design spec: Forest.Match(line)).
func (p *ProcessForest) Match(line string) Cluster {
	return p.inner.Match(p.m, line)
}

// Match parses a masked line into a template cluster for the given metadata.
func (f *Forest) Match(m meta.Meta, line string) Cluster {
	tokens := tokenize(line)
	f.mu.Lock()
	defer f.mu.Unlock()
	tr := f.treeFor(m)
	cl := tr.match(tokens)
	return Cluster{Template: cl.templateString()}
}

func (f *Forest) treeFor(m meta.Meta) *tree {
	key := m.Key()
	tr, ok := f.trees[key]
	if !ok {
		tr = newTree(f.depth, f.threshold)
		f.trees[key] = tr
	}
	return tr
}

func tokenize(line string) []string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return []string{"<EMPTY>"}
	}
	return fields
}

func newTree(maxDepth int, threshold float64) *tree {
	return &tree{
		maxDepth:  maxDepth,
		threshold: threshold,
		byLength:  make(map[int][]*logCluster),
	}
}

type tree struct {
	maxDepth  int
	threshold float64
	byLength  map[int][]*logCluster
}

func (t *tree) match(tokens []string) *logCluster {
	node := t.nodeFor(tokens)
	var best *logCluster
	bestSim := -1.0
	for _, c := range node {
		sim := jaccardSimilarity(tokens, c.template)
		if sim > bestSim {
			bestSim = sim
			best = c
		}
	}
	if best != nil && bestSim >= t.threshold {
		best.merge(tokens)
		return best
	}
	if t.totalClusters() >= maxClustersPerTree {
		t.evictOne()
	}
	c := &logCluster{template: append([]string(nil), tokens...)}
	t.byLength[len(tokens)] = append(t.byLength[len(tokens)], c)
	return c
}

func (t *tree) totalClusters() int {
	n := 0
	for _, list := range t.byLength {
		n += len(list)
	}
	return n
}

func (t *tree) evictOne() {
	for length, list := range t.byLength {
		if len(list) == 0 {
			continue
		}
		t.byLength[length] = list[1:]
		return
	}
}

func (t *tree) nodeFor(tokens []string) []*logCluster {
	// Fixed-depth routing: first (maxDepth-2) prefix tokens select a child bucket,
	// then Jaccard runs within that bucket (same length + shared prefix).
	prefixLen := t.maxDepth - 2
	if prefixLen < 0 {
		prefixLen = 0
	}
	key := prefixKey(tokens, prefixLen)
	var out []*logCluster
	for _, c := range t.byLength[len(tokens)] {
		if c.prefixKey(prefixLen) == key {
			out = append(out, c)
		}
	}
	return out
}

func prefixKey(tokens []string, n int) string {
	if n <= 0 {
		return ""
	}
	if n > len(tokens) {
		n = len(tokens)
	}
	return strings.Join(tokens[:n], "\x1f")
}

func jaccardSimilarity(a, b []string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}
	setA := make(map[string]struct{}, len(a))
	setB := make(map[string]struct{}, len(b))
	for _, tok := range a {
		setA[tok] = struct{}{}
	}
	for _, tok := range b {
		setB[tok] = struct{}{}
	}
	inter := 0
	for k := range setA {
		if _, ok := setB[k]; ok {
			inter++
		}
	}
	union := len(setA) + len(setB) - inter
	if union == 0 {
		return 1.0
	}
	return float64(inter) / float64(union)
}

type logCluster struct {
	template []string
}

func (c *logCluster) prefixKey(n int) string {
	return prefixKey(c.template, n)
}

func (c *logCluster) merge(tokens []string) {
	maxLen := len(c.template)
	if len(tokens) > maxLen {
		maxLen = len(tokens)
	}
	merged := make([]string, maxLen)
	copy(merged, c.template)
	for i := 0; i < maxLen; i++ {
		var cur, incoming string
		if i < len(merged) {
			cur = merged[i]
		}
		if i < len(tokens) {
			incoming = tokens[i]
		}
		if cur == wildcardToken || incoming == "" {
			if cur == "" && incoming != "" {
				merged[i] = incoming
			}
			continue
		}
		if cur == incoming {
			continue
		}
		merged[i] = wildcardToken
	}
	c.template = merged
}

func (c *logCluster) templateString() string {
	if len(c.template) == 0 {
		return ""
	}
	return strings.Join(c.template, " ")
}
