package main

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

const (
	// maxAliasDepth guards against malformed cyclic YAML alias chains.
	maxAliasDepth = 64
	// maxLCSCells bounds the LCS DP table size so pathological diffs fail conservatively.
	maxLCSCells = 4_000_000
)

// Mismatch describes a single semantic discrepancy detected after merging.
type Mismatch struct {
	Path          string
	Reason        string
	DifferingKeys []string
}

// String formats a Mismatch for human-readable diagnostics.
func (m Mismatch) String() string {
	if len(m.DifferingKeys) > 0 {
		return fmt.Sprintf("%s: %s (differing keys: %s)", m.Path, m.Reason, strings.Join(m.DifferingKeys, ", "))
	}
	return fmt.Sprintf("%s: %s", m.Path, m.Reason)
}

// VerifyResult summarizes the outcome of post-merge semantic YAML verification.
type VerifyResult struct {
	Failures []Mismatch
	Touched  []string
	Diverged []string
}

// VerifyMerge parses local, PR base, PR head, and merged OpenAPI specs and verifies
// that every untouched path/component still equals local, every non-diverged
// PR-touched path/component equals PR head, and diverged entries preserve both
// sides cleanly without introducing duplicate YAML keys.
func VerifyMerge(localBytes, baseBytes, headBytes, mergedBytes []byte) (*VerifyResult, error) {
	localDoc, err := parseYAMLDoc(localBytes, "local spec")
	if err != nil {
		return nil, err
	}
	baseDoc, err := parseYAMLDoc(baseBytes, "PR base spec")
	if err != nil {
		return nil, err
	}
	headDoc, err := parseYAMLDoc(headBytes, "PR head spec")
	if err != nil {
		return nil, err
	}
	mergedDoc, err := parseYAMLDoc(mergedBytes, "merged spec")
	if err != nil {
		return nil, err
	}

	res := &VerifyResult{}

	localDups := collectDuplicateKeys(localDoc)
	mergedDups := collectDuplicateKeys(mergedDoc)
	for path, keys := range mergedDups {
		existing := localDups[path]
		for key := range keys {
			if existing[key] {
				continue
			}
			dupPath := key
			if path != "" {
				dupPath = path + "." + key
			}
			res.Failures = append(res.Failures, Mismatch{
				Path:   dupPath,
				Reason: "duplicate YAML mapping key introduced by merge",
			})
		}
	}

	verifyNode(nil, localDoc, baseDoc, headDoc, mergedDoc, res)

	slices.Sort(res.Touched)
	res.Touched = slices.Compact(res.Touched)
	slices.Sort(res.Diverged)
	res.Diverged = slices.Compact(res.Diverged)
	slices.SortFunc(res.Failures, func(a, b Mismatch) int {
		if c := strings.Compare(a.Path, b.Path); c != 0 {
			return c
		}
		return strings.Compare(a.Reason, b.Reason)
	})

	return res, nil
}

func parseYAMLDoc(data []byte, label string) (*yaml.Node, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parsing %s as YAML: %w", label, err)
	}
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return nil, fmt.Errorf("parsing %s: expected YAML document node", label)
	}
	return root.Content[0], nil
}

func verifyNode(parts []string, l, b, h, m *yaml.Node, res *VerifyResult) {
	l = deref(l)
	b = deref(b)
	h = deref(h)
	m = deref(m)

	if isContainer(parts) && allNilOrMapping(l, b, h, m) {
		recurseMappings(parts, l, b, h, m, res)
		return
	}

	path := formatPath(parts)

	// Case 1: Entry was not touched by the PR — merged result must match local.
	if nodesEqual(b, h) {
		if !nodesEqual(m, l) {
			res.Failures = append(res.Failures, Mismatch{
				Path:          path,
				Reason:        "untouched by PR, but modified by merge",
				DifferingKeys: diffMappingKeys(l, m),
			})
		}
		return
	}

	// Case 2: PR touched this entry and local matched PR base — merged must match PR head.
	if nodesEqual(l, b) {
		if nodesEqual(m, h) {
			if isEntryLevel(parts) {
				res.Touched = append(res.Touched, path)
			}
			return
		}
		res.Failures = append(res.Failures, Mismatch{
			Path:          path,
			Reason:        "expected PR head version (local matched PR base), but merged result differs",
			DifferingKeys: diffMappingKeys(h, m),
		})
		return
	}

	// Case 3: PR touched this entry and local already matched PR head (e.g., re-apply).
	if nodesEqual(l, h) {
		if nodesEqual(m, h) {
			if isEntryLevel(parts) {
				res.Touched = append(res.Touched, path)
			}
			return
		}
		res.Failures = append(res.Failures, Mismatch{
			Path:          path,
			Reason:        "local already matched PR head, but merged result differs",
			DifferingKeys: diffMappingKeys(h, m),
		})
		return
	}

	// Case 4: Both local and the PR's base diverged independently on this entry.
	if isEntryLevel(parts) {
		res.Touched = append(res.Touched, path)
		res.Diverged = append(res.Diverged, path)
	}

	if allNonNilKind(yaml.MappingNode, l, b, h, m) {
		recurseMappings(parts, l, b, h, m, res)
		return
	}

	// Leaf scalar, sequence, or add/delete collision where both sides differed from base.
	// Validate exact delta preservation (including idempotent reapplication) and fail
	// conservatively whenever unambiguous preservation cannot be established.
	verifyDivergedLeaf(path, l, b, h, m, res)
}

func verifyDivergedLeaf(path string, l, b, h, m *yaml.Node, res *VerifyResult) {
	if allNonNilKind(yaml.SequenceNode, l, b, h, m) {
		expected, ok := threeWayMerge(l.Content, b.Content, h.Content, nodesEqual)
		if !ok {
			res.Failures = append(res.Failures, Mismatch{
				Path:   path,
				Reason: "cannot verify diverged sequence merge safely (overlapping local and PR changes)",
			})
			return
		}
		if !slices.EqualFunc(m.Content, expected, nodesEqual) {
			res.Failures = append(res.Failures, Mismatch{
				Path:   path,
				Reason: "diverged sequence merge did not preserve both local and PR changes",
			})
		}
		return
	}

	if allNonNilStringScalar(l, b, h, m) {
		var split func(string) []string
		if strings.Contains(b.Value, "\n") || strings.Contains(l.Value, "\n") ||
			strings.Contains(h.Value, "\n") || strings.Contains(m.Value, "\n") {
			split = func(s string) []string { return strings.Split(s, "\n") }
		} else {
			split = splitWhitespaceTokens
		}
		expected, ok := threeWayMerge(split(l.Value), split(b.Value), split(h.Value), func(a, b string) bool { return a == b })
		if !ok {
			res.Failures = append(res.Failures, Mismatch{
				Path:   path,
				Reason: "cannot verify diverged scalar merge safely (overlapping local and PR changes)",
			})
			return
		}
		if !slices.Equal(split(m.Value), expected) {
			res.Failures = append(res.Failures, Mismatch{
				Path:   path,
				Reason: "diverged scalar merge did not preserve both local and PR changes",
			})
		}
		return
	}

	res.Failures = append(res.Failures, Mismatch{
		Path:   path,
		Reason: "cannot verify diverged merge safely (both local and PR modified this entry)",
	})
}

func allNonNilKind(kind yaml.Kind, nodes ...*yaml.Node) bool {
	for _, n := range nodes {
		if n == nil || n.Kind != kind {
			return false
		}
	}
	return true
}

func allNonNilStringScalar(nodes ...*yaml.Node) bool {
	for _, n := range nodes {
		if n == nil || n.Kind != yaml.ScalarNode || n.ShortTag() != "!!str" {
			return false
		}
	}
	return true
}

type editHunk[T any] struct {
	bStart int
	bEnd   int
	repl   []T
}

// diffHunks computes minimal LCS-based edit hunks transforming base into other.
// It returns ok=false if the trimmed diff exceeds maxLCSCells.
func diffHunks[T any](base, other []T, eq func(T, T) bool) ([]editHunk[T], bool) {
	pre := 0
	for pre < len(base) && pre < len(other) && eq(base[pre], other[pre]) {
		pre++
	}
	bEnd := len(base)
	oEnd := len(other)
	for bEnd > pre && oEnd > pre && eq(base[bEnd-1], other[oEnd-1]) {
		bEnd--
		oEnd--
	}
	if pre == bEnd && pre == oEnd {
		return nil, true
	}

	bMid := base[pre:bEnd]
	oMid := other[pre:oEnd]
	if len(bMid) == 0 || len(oMid) == 0 {
		return []editHunk[T]{{bStart: pre, bEnd: bEnd, repl: oMid}}, true
	}
	if len(bMid)*len(oMid) > maxLCSCells {
		return nil, false
	}

	rows := len(bMid) + 1
	cols := len(oMid) + 1
	dp := make([]int, rows*cols)
	for i, bVal := range slices.Backward(bMid) {
		for j, oVal := range slices.Backward(oMid) {
			if eq(bVal, oVal) {
				dp[i*cols+j] = dp[(i+1)*cols+(j+1)] + 1
			} else {
				dp[i*cols+j] = max(dp[(i+1)*cols+j], dp[i*cols+(j+1)])
			}
		}
	}

	type matchPair struct{ b, o int }
	var matches []matchPair
	for i, j := 0, 0; i < len(bMid) && j < len(oMid); {
		switch {
		case eq(bMid[i], oMid[j]):
			matches = append(matches, matchPair{b: pre + i, o: pre + j})
			i++
			j++
		case dp[(i+1)*cols+j] >= dp[i*cols+(j+1)]:
			i++
		default:
			j++
		}
	}

	var hunks []editHunk[T]
	curB, curO := pre, pre
	for _, m := range matches {
		if curB < m.b || curO < m.o {
			hunks = append(hunks, editHunk[T]{
				bStart: curB,
				bEnd:   m.b,
				repl:   other[curO:m.o],
			})
		}
		curB = m.b + 1
		curO = m.o + 1
	}
	if curB < bEnd || curO < oEnd {
		hunks = append(hunks, editHunk[T]{
			bStart: curB,
			bEnd:   bEnd,
			repl:   other[curO:oEnd],
		})
	}
	return hunks, true
}

// threeWayMerge merges non-overlapping edits from base->l and base->h.
// Identical hunks on both sides (such as idempotent reapplication) are
// deduplicated cleanly; overlapping or touching non-identical hunks return ok=false.
func threeWayMerge[T any](l, b, h []T, eq func(T, T) bool) ([]T, bool) {
	lHunks, ok := diffHunks(b, l, eq)
	if !ok {
		return nil, false
	}
	hHunks, ok := diffHunks(b, h, eq)
	if !ok {
		return nil, false
	}

	var out []T
	bPos, li, hi := 0, 0, 0
	for li < len(lHunks) && hi < len(hHunks) {
		lh := lHunks[li]
		hh := hHunks[hi]
		switch {
		case lh.bEnd < hh.bStart:
			out = append(out, b[bPos:lh.bStart]...)
			out = append(out, lh.repl...)
			bPos = lh.bEnd
			li++
		case hh.bEnd < lh.bStart:
			out = append(out, b[bPos:hh.bStart]...)
			out = append(out, hh.repl...)
			bPos = hh.bEnd
			hi++
		case lh.bStart == hh.bStart && lh.bEnd == hh.bEnd && slices.EqualFunc(lh.repl, hh.repl, eq):
			out = append(out, b[bPos:lh.bStart]...)
			out = append(out, lh.repl...)
			bPos = lh.bEnd
			li++
			hi++
		default:
			return nil, false
		}
	}
	for ; li < len(lHunks); li++ {
		lh := lHunks[li]
		out = append(out, b[bPos:lh.bStart]...)
		out = append(out, lh.repl...)
		bPos = lh.bEnd
	}
	for ; hi < len(hHunks); hi++ {
		hh := hHunks[hi]
		out = append(out, b[bPos:hh.bStart]...)
		out = append(out, hh.repl...)
		bPos = hh.bEnd
	}
	out = append(out, b[bPos:]...)
	return out, true
}

// splitWhitespaceTokens splits a string into alternating whitespace and
// non-whitespace runs so single-line folded strings preserve exact spacing.
func splitWhitespaceTokens(s string) []string {
	if s == "" {
		return nil
	}
	var tokens []string
	start := 0
	firstRune, _ := utf8.DecodeRuneInString(s)
	inSpace := unicode.IsSpace(firstRune)
	for i, r := range s {
		space := unicode.IsSpace(r)
		if space != inSpace {
			tokens = append(tokens, s[start:i])
			start = i
			inSpace = space
		}
	}
	tokens = append(tokens, s[start:])
	return tokens
}

func recurseMappings(parts []string, l, b, h, m *yaml.Node, res *VerifyResult) {
	lMap := mappingValues(l)
	bMap := mappingValues(b)
	hMap := mappingValues(h)
	mMap := mappingValues(m)

	keys := unionKeys(lMap, bMap, hMap, mMap)
	for _, k := range keys {
		childParts := append(slices.Clone(parts), k)
		verifyNode(childParts, lMap[k], bMap[k], hMap[k], mMap[k], res)
	}
}

// isContainer returns true for structural OpenAPI containers whose children
// are individual paths or component definitions.
func isContainer(parts []string) bool {
	switch len(parts) {
	case 0:
		return true
	case 1:
		return parts[0] == "paths" || parts[0] == "components"
	case 2:
		return parts[0] == "components"
	default:
		return false
	}
}

// isEntryLevel returns true for paths representing an individual OpenAPI path,
// component item (schema, response, parameter, etc.), or top-level section.
func isEntryLevel(parts []string) bool {
	switch len(parts) {
	case 1:
		return parts[0] != "paths" && parts[0] != "components"
	case 2:
		return parts[0] == "paths"
	case 3:
		return parts[0] == "components"
	default:
		return false
	}
}

func formatPath(parts []string) string {
	if len(parts) == 0 {
		return "<root>"
	}
	var b strings.Builder
	for i, p := range parts {
		if i == 0 {
			b.WriteString(p)
			continue
		}
		if (i == 1 && parts[0] == "paths") || strings.ContainsAny(p, "./[] ") {
			fmt.Fprintf(&b, "[%q]", p)
		} else {
			b.WriteByte('.')
			b.WriteString(p)
		}
	}
	return b.String()
}

func deref(n *yaml.Node) *yaml.Node {
	for range maxAliasDepth {
		if n == nil || n.Kind != yaml.AliasNode {
			return n
		}
		n = n.Alias
	}
	return n
}

func allNilOrMapping(nodes ...*yaml.Node) bool {
	hasMapping := false
	for _, n := range nodes {
		if n == nil {
			continue
		}
		if n.Kind != yaml.MappingNode {
			return false
		}
		hasMapping = true
	}
	return hasMapping
}

func mappingValues(n *yaml.Node) map[string]*yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	vals := make(map[string]*yaml.Node, len(n.Content)/2)
	for i := 0; i+1 < len(n.Content); i += 2 {
		vals[n.Content[i].Value] = n.Content[i+1]
	}
	return vals
}

func unionKeys(maps ...map[string]*yaml.Node) []string {
	seen := make(map[string]struct{})
	for _, m := range maps {
		for k := range m {
			seen[k] = struct{}{}
		}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func nodesEqual(a, b *yaml.Node) bool {
	a = deref(a)
	b = deref(b)
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case yaml.ScalarNode:
		tagA := a.ShortTag()
		tagB := b.ShortTag()
		if tagA != tagB {
			return false
		}
		if tagA == "!!null" {
			return true
		}
		return a.Value == b.Value
	case yaml.SequenceNode:
		if len(a.Content) != len(b.Content) {
			return false
		}
		for i := range a.Content {
			if !nodesEqual(a.Content[i], b.Content[i]) {
				return false
			}
		}
		return true
	case yaml.MappingNode:
		aMap := mappingValues(a)
		bMap := mappingValues(b)
		if len(aMap) != len(bMap) {
			return false
		}
		for k, av := range aMap {
			bv, ok := bMap[k]
			if !ok || !nodesEqual(av, bv) {
				return false
			}
		}
		return true
	default:
		return a.Value == b.Value
	}
}

func diffMappingKeys(a, b *yaml.Node) []string {
	a = deref(a)
	b = deref(b)
	if a == nil || b == nil || a.Kind != yaml.MappingNode || b.Kind != yaml.MappingNode {
		return nil
	}
	aMap := mappingValues(a)
	bMap := mappingValues(b)
	keys := unionKeys(aMap, bMap)
	var diff []string
	for _, k := range keys {
		if !nodesEqual(aMap[k], bMap[k]) {
			diff = append(diff, k)
		}
	}
	return diff
}

func collectDuplicateKeys(root *yaml.Node) map[string]map[string]bool {
	dups := make(map[string]map[string]bool)
	var walk func(parts []string, n *yaml.Node)
	walk = func(parts []string, n *yaml.Node) {
		if n == nil || n.Kind == yaml.AliasNode {
			return
		}
		switch n.Kind {
		case yaml.MappingNode:
			seen := make(map[string]bool, len(n.Content)/2)
			for i := 0; i+1 < len(n.Content); i += 2 {
				k := n.Content[i].Value
				if seen[k] {
					path := ""
					if len(parts) > 0 {
						path = formatPath(parts)
					}
					if dups[path] == nil {
						dups[path] = make(map[string]bool)
					}
					dups[path][k] = true
				}
				seen[k] = true
				walk(append(slices.Clone(parts), k), n.Content[i+1])
			}
		case yaml.SequenceNode:
			for i, item := range n.Content {
				walk(append(slices.Clone(parts), fmt.Sprintf("[%d]", i)), item)
			}
		}
	}
	walk(nil, root)
	return dups
}
