package drain

import (
	"strings"
	"sync"

	"github.com/OrenVill/Argus/internal/meta"
)

const wildcardToken = "<*>"

// Cluster is the result of matching a masked log line.
type Cluster struct {
	Template string
}

// Forest holds Drain trees keyed by (env, service).
type Forest struct {
	depth     int
	threshold float64
	mu        sync.RWMutex
	trees     map[string]*tree
}

// NewForest creates a forest with the given depth and similarity threshold.
func NewForest(depth int, threshold float64) *Forest {
	if depth <= 0 {
		depth = 4
	}
	if threshold <= 0 {
		threshold = 0.5
	}
	return &Forest{
		depth:     depth,
		threshold: threshold,
		trees:     make(map[string]*tree),
	}
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
		sim := jaccardSimilarity(tokens, c.representative())
		if sim > bestSim {
			bestSim = sim
			best = c
		}
	}
	if best != nil && bestSim >= t.threshold {
		best.add(tokens)
		return best
	}
	c := &logCluster{tokens: [][]string{append([]string(nil), tokens...)}}
	t.byLength[len(tokens)] = append(t.byLength[len(tokens)], c)
	return c
}

func (t *tree) nodeFor(tokens []string) []*logCluster {
	// Fixed-depth routing: first (maxDepth-1) prefix tokens select a child bucket,
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
	tokens [][]string
}

func (c *logCluster) prefixKey(n int) string {
	return prefixKey(c.representative(), n)
}

func (c *logCluster) representative() []string {
	if len(c.tokens) == 0 {
		return nil
	}
	return c.tokens[0]
}

func (c *logCluster) add(tokens []string) {
	c.tokens = append(c.tokens, append([]string(nil), tokens...))
}

func (c *logCluster) templateString() string {
	if len(c.tokens) == 0 {
		return ""
	}
	maxLen := 0
	for _, seq := range c.tokens {
		if len(seq) > maxLen {
			maxLen = len(seq)
		}
	}
	out := make([]string, maxLen)
	for i := 0; i < maxLen; i++ {
		var first string
		wildcard := false
		for _, seq := range c.tokens {
			if i >= len(seq) {
				wildcard = true
				continue
			}
			tok := seq[i]
			if first == "" {
				first = tok
				continue
			}
			if tok != first {
				wildcard = true
			}
		}
		if wildcard || first == "" {
			out[i] = wildcardToken
		} else {
			out[i] = first
		}
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return strings.Join(out, " ")
}
