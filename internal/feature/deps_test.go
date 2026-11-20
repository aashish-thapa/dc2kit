package feature

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/aashish-thapa/dc2kit/internal/devcontainer"
)

// fakeResolver serves Features from a fixed table and records what it was
// asked for.
type fakeResolver struct {
	table map[string]Feature
	asked []devcontainer.FeatureRequest
}

func (f *fakeResolver) Resolve(_ context.Context, req devcontainer.FeatureRequest) (Feature, error) {
	f.asked = append(f.asked, req)
	got, ok := f.table[NormalizeKey(req.Ref)]
	if !ok {
		return Feature{}, fmt.Errorf("no feature %s", req.Ref)
	}
	got.Request = req
	return got, nil
}

// dependsOn builds a Feature whose dependsOn is the JSON object deps.
func dependsOn(key, deps string) Feature {
	f := Feature{Key: key, Request: devcontainer.FeatureRequest{Ref: key + ":1"}, Metadata: Metadata{ID: key}}
	if deps != "" {
		if err := json.Unmarshal([]byte(deps), &f.Metadata.DependsOn); err != nil {
			panic(err)
		}
	}
	return f
}

func TestWithDependenciesResolvesUnlistedDependsOnTransitively(t *testing.T) {
	r := &fakeResolver{table: map[string]Feature{
		"r/rust":   dependsOn("r/rust", `{"r/common:2": {}}`),
		"r/common": dependsOn("r/common", ""),
	}}
	local := dependsOn("./rust", `{"r/rust:1": {"version": "nightly", "profile": "minimal"}}`)

	got, err := WithDependencies(t.Context(), r, []Feature{local}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if keys(got) != "./rust,r/rust,r/common" {
		t.Fatalf("features = %s", keys(got))
	}
	opts := r.asked[0].Options
	if len(opts) != 2 || opts[0] != (devcontainer.Option{Name: "version", Value: "nightly"}) || opts[1].Value != "minimal" {
		t.Fatalf("dependency options = %+v", opts)
	}
	if _, err := Order(got, nil); err != nil {
		t.Fatalf("the closed set must order: %v", err)
	}
}

func TestWithDependenciesLetsAListedFeatureSatisfyADependency(t *testing.T) {
	r := &fakeResolver{}
	listed := []Feature{dependsOn("r/a", `{"r/b:1": {}}`), dependsOn("r/b", `{"r/a": {}}`)}
	listed[1].Request.Ref = "r/b:2"
	got, err := WithDependencies(t.Context(), r, listed, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.asked) != 0 || len(got) != 2 {
		t.Fatalf("resolved %d extra, got %s", len(r.asked), keys(got))
	}
}

func TestWithDependenciesNamesTheDependentOnFailure(t *testing.T) {
	_, err := WithDependencies(t.Context(), &fakeResolver{}, []Feature{dependsOn("./x", `{"r/gone:1": {}}`)}, nil)
	if err == nil || !strings.Contains(err.Error(), `"./x:1"`) {
		t.Fatalf("want an error naming the dependent feature, got %v", err)
	}
}
