package feature

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aashish-thapa/dc2kit/internal/devcontainer"
)

// Dependencies returns the Features this one dependsOn, with the options it
// asks for. Keys come back sorted, so the result is stable.
func (m Metadata) Dependencies() ([]devcontainer.FeatureRequest, error) {
	if len(m.DependsOn) == 0 {
		return nil, nil
	}
	doc, err := json.Marshal(m.DependsOn)
	if err != nil {
		return nil, err
	}
	return devcontainer.ParseFeatureRequests(doc)
}

// WithDependencies adds every Feature that a resolved one dependsOn but the
// config does not list, transitively. The spec installs those
// automatically; matching is by Key, so a listed Feature satisfies a
// dependency whatever its tag. onResolve, if set, is told about each added
// request before it is resolved.
func WithDependencies(ctx context.Context, r Resolver, features []Feature, onResolve func(devcontainer.FeatureRequest)) ([]Feature, error) {
	all := append([]Feature(nil), features...)
	present := make(map[string]bool, len(all))
	for _, f := range all {
		present[f.Key] = true
	}
	for i := 0; i < len(all); i++ {
		deps, err := all[i].Metadata.Dependencies()
		if err != nil {
			return nil, fmt.Errorf("feature %q dependsOn: %w", all[i].Request.Ref, err)
		}
		for _, req := range deps {
			key := NormalizeKey(req.Ref)
			if present[key] {
				continue
			}
			present[key] = true
			if onResolve != nil {
				onResolve(req)
			}
			f, err := r.Resolve(ctx, req)
			if err != nil {
				return nil, fmt.Errorf("feature %q dependsOn: %w", all[i].Request.Ref, err)
			}
			all = append(all, f)
		}
	}
	return all, nil
}
