module github.com/aashish-thapa/dc2kit

go 1.27.4

require (
	github.com/docker/sandbox-kit-spec/v3 v3.0.0-m.9
	github.com/opencontainers/image-spec v1.1.1
	github.com/tailscale/hujson v0.0.0-20260727124030-b80ff77dac4f
	gopkg.in/yaml.v3 v3.0.1
	oras.land/oras-go/v2 v2.6.3
)

require (
	github.com/distribution/reference v0.6.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
)

replace github.com/docker/sandbox-kit-spec/v3 => ../sandbox-kit-spec
