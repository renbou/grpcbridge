package httprule

import "strings"

// Trie is the main data structure used for pattern-based routing with HttpRule templates.
// It is a simple trie grouped by HTTP method and represented as a nested map,
// with extra handling for different component types such as wildcards.
// After a trie is set up using Add(), it is safe to call Find() concurrently,
// as it does not make any modifications to the internal structure.
type Trie struct {
	methods map[string]*node
}

// NewTrie initializes a new [Trie].
func NewTrie() *Trie {
	return &Trie{
		methods: make(map[string]*node),
	}
}

// Add adds a new HttpRule [Template] to the trie, attaching it to the specified HTTP method.
func (t *Trie) Add(method string, tmpl *Template) {
	if t.methods[method] == nil {
		t.methods[method] = new(node)
	}

	n := t.methods[method]

	for _, s := range tmpl.segments {
		n = n.add(s)
	}

	n.addVerb(tmpl)
}

// Find finds the appropriate [Template] for the given method and path, and returns false if no match was found.
func (t *Trie) Find(method string, path string) (*Template, bool) {
	n, ok := t.methods[method]
	if !ok {
		return nil, false
	}

	path = strings.TrimPrefix(path, "/")

	return dfs(n, path, path, false)
}

func dfs(n *node, orig, path string, last bool) (*Template, bool) {
	// if this is the end, find the appropriate template without going deeper
	if last {
		return dfsLeaf(n, orig)
	}

	component := path
	slash := strings.Index(path, "/")
	if slash != -1 {
		component, path = path[:slash], path[slash+1:]
	}

	// try matching literal first since its the most specific
	if n.literals[component] != nil {
		return dfs(n.literals[component], orig, path, slash == -1)
	}

	// then literal with verb if this is the last component
	if slash == -1 {
		var verb string

		for verbIdx := strings.LastIndex(component, ":"); verbIdx != -1; verbIdx = strings.LastIndex(component, ":") {
			verb, component = component[verbIdx+1:], component[:verbIdx]
			if next := n.literals[component]; next != nil && next.verbs[verb] != nil {
				return next.verbs[verb].tmpl, true
			}
		}
	}

	// then wildcards
	if n.wildcard != nil {
		return dfs(n.wildcard, orig, path, slash == -1)
	}

	if n.multiWildcard != nil {
		return dfs(n.multiWildcard, orig, "", true)
	}

	return nil, false
}

func dfsLeaf(n *node, orig string) (*Template, bool) {
	if n.tmpl != nil {
		return n.tmpl, true
	}

	// wildcards might've consumed the verb, additionally check if the original path matches any verb here
	for verb, next := range n.verbs {
		if strings.HasSuffix(orig, ":"+verb) {
			return next.tmpl, true
		}
	}

	return nil, false
}

type node struct {
	// leaf node
	tmpl *Template

	// non-leaf node
	literals      map[string]*node
	verbs         map[string]*node
	wildcard      *node
	multiWildcard *node
}

func (n *node) add(s segment) *node {
	switch s.typ {
	case segmentWildcard:
		return n.addWildcard()
	case segmentMultiWildcard:
		return n.addMultiWildcard()
	case segmentVariable:
		// guaranteed by Parse to not be recursive
		return n.addVariable(s)
	default:
	}

	return n.addLiteral(s)
}

func (n *node) addVariable(s segment) *node {
	res := n

	for _, s := range s.variable.segments {
		res = res.add(s)
	}

	return res
}

func (n *node) addLiteral(s segment) *node {
	if n.literals == nil {
		n.literals = make(map[string]*node)
	}

	if n.literals[s.literal] == nil {
		n.literals[s.literal] = new(node)
	}

	return n.literals[s.literal]
}

func (n *node) addWildcard() *node {
	if n.wildcard == nil {
		n.wildcard = new(node)
	}

	return n.wildcard
}

func (n *node) addMultiWildcard() *node {
	if n.multiWildcard == nil {
		n.multiWildcard = new(node)
	}

	return n.multiWildcard
}

// either set this node as leaf if verb is empty, or add this node to the verb map
func (n *node) addVerb(tmpl *Template) {
	if tmpl.verb == "" {
		n.tmpl = tmpl
		return
	}

	if n.verbs == nil {
		n.verbs = make(map[string]*node)
	}

	// always a leaf node
	n.verbs[tmpl.verb] = &node{tmpl: tmpl}
}
