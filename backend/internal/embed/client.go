// Package embed provides a pluggable embedding system for converting text into
// vector representations (arrays of floating-point numbers). These vectors
// capture the semantic meaning of the text and can be compared using cosine
// similarity to find related content.
//
// The package defines an Embedder interface and provides three implementations,
// selected at startup via the factory function NewEmbedder:
//
//   - "local"  -- LocalEmbedder: calls a sidecar HTTP service running on the
//     same machine (e.g. a Python sentence-transformers server).
//   - "api"    -- APIEmbedder: calls an external embedding API over the internet
//     (e.g. OpenAI's /v1/embeddings endpoint).
//   - "noop"   -- NoopEmbedder: returns zero vectors; used for testing or when
//     embedding is disabled.
//
// This is the "factory pattern": the caller asks for an Embedder by name and
// gets back the right implementation without needing to know the concrete type.
package embed

import (
	"context"
	"fmt"
)

// Embedder is the interface that all embedding backends must implement. It
// abstracts away the details of how text is converted to vectors, allowing the
// rest of the application to work with any backend interchangeably.
type Embedder interface {
	// Embed returns the embedding vector for a single text input.
	// The returned slice has length equal to Dimensions().
	Embed(ctx context.Context, text string) ([]float32, error)
	// EmbedBatch returns embedding vectors for multiple text inputs in one call.
	// This is more efficient than calling Embed in a loop because it can batch
	// the network request.
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	// Dimensions returns the dimensionality of the embedding vectors (e.g. 384
	// for MiniLM, 1536 for OpenAI ada-002). All vectors from a given embedder
	// have the same number of dimensions.
	Dimensions() int
}

// NewEmbedder is a factory function that creates the appropriate Embedder
// implementation based on the backend name. This is the single entry point for
// creating embedders — callers do not need to know about LocalEmbedder,
// APIEmbedder, or NoopEmbedder directly.
//
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
