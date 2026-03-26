package embed

import (
	"context"
	"fmt"
)

// Embedder generates vector embeddings from text.
type Embedder interface {
	// Embed returns the embedding vector for a single text input.
	Embed(ctx context.Context, text string) ([]float32, error)
	// EmbedBatch returns embedding vectors for multiple text inputs.
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	// Dimensions returns the dimensionality of the embedding vectors.
	Dimensions() int
}

// NewEmbedder creates an Embedder based on the specified backend.
// Supported backends: "local", "api", "noop".
func NewEmbedder(backend, model, apiURL, apiKey string, dims int) (Embedder, error) {
	switch backend {
	case "local":
		return NewLocalEmbedder(apiURL, model, dims)
	case "api":
		return NewAPIEmbedder(apiURL, apiKey, model, dims)
	case "noop":
		return NewNoopEmbedder(dims), nil
	default:
		return nil, fmt.Errorf("unknown embedding backend: %s", backend)
	}
}
