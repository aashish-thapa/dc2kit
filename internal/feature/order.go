package feature

import (
	"fmt"
	"sort"
	"strings"
)

// Order returns features in install order. `dependsOn` is a hard edge and
// must name a feature present in the set; `installsAfter` is a soft edge
// honored only when its target is present. Among features free to install,
// those listed in override go first in that order, then the rest in
// document order — the devcontainer CLI's tie-break.
func Order(features []Feature, override []string) ([]Feature, error) {
	byKey := make(map[string]int, len(features))
	for i, f := range features {
		if prev, dup := byKey[f.Key]; dup {
			return nil, fmt.Errorf("features %q and %q are the same feature", features[prev].Request.Ref, f.Request.Ref)
		}
		byKey[f.Key] = i
	}

	priority := make(map[int]int, len(override))
	for rank, ref := range override {
		if i, ok := byKey[NormalizeKey(ref)]; ok {
			priority[i] = rank
		}
	}

	after := make([][]int, len(features))
	pending := make([]int, len(features))
	for i, f := range features {
		for dep := range f.Metadata.DependsOn {
			j, ok := byKey[NormalizeKey(dep)]
			if !ok {
				return nil, fmt.Errorf("feature %q depends on %q, which was not resolved", f.Request.Ref, dep)
			}
			after[j] = append(after[j], i)
			pending[i]++
		}
		for _, soft := range f.Metadata.InstallsAfter {
			if j, ok := byKey[NormalizeKey(soft)]; ok && j != i {
				after[j] = append(after[j], i)
				pending[i]++
			}
		}
	}

	less := func(a, b int) bool {
		pa, aRanked := priority[a]
		pb, bRanked := priority[b]
		switch {
		case aRanked && bRanked:
			return pa < pb
		case aRanked != bRanked:
			return aRanked
		default:
			return a < b
		}
	}

	var ready []int
	for i := range features {
		if pending[i] == 0 {
			ready = append(ready, i)
		}
	}
	ordered := make([]Feature, 0, len(features))
	for len(ready) > 0 {
		sort.Slice(ready, func(x, y int) bool { return less(ready[x], ready[y]) })
		next := ready[0]
		ready = ready[1:]
		ordered = append(ordered, features[next])
		for _, dependent := range after[next] {
			pending[dependent]--
			if pending[dependent] == 0 {
				ready = append(ready, dependent)
			}
		}
	}

	if len(ordered) != len(features) {
		var stuck []string
		for i, n := range pending {
			if n > 0 {
				stuck = append(stuck, features[i].Request.Ref)
			}
		}
		return nil, fmt.Errorf("features form a dependency cycle: %s", strings.Join(stuck, ", "))
	}
	return ordered, nil
}
