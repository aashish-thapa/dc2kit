package feature

import (
	"context"
	"encoding/json"
	"fmt"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"

	"github.com/aashish-thapa/dc2kit/internal/devcontainer"
)

const featureLayerMediaType = "application/vnd.devcontainers.layer.v1+tar"

// RegistryResolver fetches features published as OCI artifacts, the form
// every current feature collection (ghcr.io/devcontainers/features, …)
// uses. Access is anonymous; private feature registries are out of scope.
type RegistryResolver struct{}

// Resolve pulls the feature manifest and its single payload layer. The
// returned Pin is the manifest digest, so a generated kit records exactly
// which bytes it installs even when the tag later moves.
func (RegistryResolver) Resolve(ctx context.Context, req devcontainer.FeatureRequest) (Feature, error) {
	ref, err := registry.ParseReference(req.Ref)
	if err != nil {
		return Feature{}, fmt.Errorf("feature %q is not a registry reference: %w", req.Ref, err)
	}
	if ref.Reference == "" {
		ref.Reference = "latest"
	}

	repo, err := remote.NewRepository(ref.Registry + "/" + ref.Repository)
	if err != nil {
		return Feature{}, fmt.Errorf("feature %q: %w", req.Ref, err)
	}
	repo.Client = &auth.Client{Client: retry.DefaultClient, Cache: auth.NewCache()}

	manifestDesc, manifestBytes, err := fetchManifest(ctx, repo, ref.Reference)
	if err != nil {
		return Feature{}, fmt.Errorf("feature %q: %w", req.Ref, err)
	}
	var manifest ocispec.Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return Feature{}, fmt.Errorf("feature %q manifest: %w", req.Ref, err)
	}
	layer, err := payloadLayer(manifest)
	if err != nil {
		return Feature{}, fmt.Errorf("feature %q: %w", req.Ref, err)
	}
	blob, err := content.FetchAll(ctx, repo.Blobs(), layer)
	if err != nil {
		return Feature{}, fmt.Errorf("feature %q payload: %w", req.Ref, err)
	}

	return assemble(req, NormalizeKey(req.Ref), manifestDesc.Digest.String(), blob)
}

func fetchManifest(ctx context.Context, repo *remote.Repository, reference string) (ocispec.Descriptor, []byte, error) {
	desc, rc, err := repo.Manifests().FetchReference(ctx, reference)
	if err != nil {
		return ocispec.Descriptor{}, nil, fmt.Errorf("fetch manifest: %w", err)
	}
	defer rc.Close()
	data, err := content.ReadAll(rc, desc)
	if err != nil {
		return ocispec.Descriptor{}, nil, fmt.Errorf("read manifest: %w", err)
	}
	return desc, data, nil
}

func payloadLayer(m ocispec.Manifest) (ocispec.Descriptor, error) {
	for _, layer := range m.Layers {
		if layer.MediaType == featureLayerMediaType {
			if layer.Size > maxPayloadBytes {
				return ocispec.Descriptor{}, fmt.Errorf("payload layer is %d bytes, over the %d limit", layer.Size, maxPayloadBytes)
			}
			return layer, nil
		}
	}
	return ocispec.Descriptor{}, fmt.Errorf("manifest has no %s layer; is this a devcontainer feature?", featureLayerMediaType)
}

func assemble(req devcontainer.FeatureRequest, key, pin string, blob []byte) (Feature, error) {
	files, err := extractArchive(blob)
	if err != nil {
		return Feature{}, fmt.Errorf("feature %q: %w", req.Ref, err)
	}
	return fromFiles(req, key, pin, files)
}

func fromFiles(req devcontainer.FeatureRequest, key, pin string, files []File) (Feature, error) {
	meta, err := metadataFrom(files)
	if err != nil {
		return Feature{}, fmt.Errorf("feature %q: %w", req.Ref, err)
	}
	if !hasInstallScript(files) {
		return Feature{}, fmt.Errorf("feature %q: payload has no install.sh", req.Ref)
	}
	return Feature{Request: req, Key: key, Pin: pin, Metadata: meta, Files: files}, nil
}
