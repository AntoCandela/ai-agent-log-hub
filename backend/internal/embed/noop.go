package embed

import "context"

// NoopEmbedder returns zero vectors. It exists for two reasons:
//
//  1. Testing: tests that exercise the summary pipeline need an Embedder but
//     do not care about actual embedding values. NoopEmbedder satisfies the
//     interface without requiring a running sidecar or API key.
//
//  2. Disabled embedding: in production deployments where semantic search is
//     not needed, the operator can set backend="noop" to skip real embedding
//     while still writing valid (all-zeros) vectors to the database. This
//     keeps the rest of the code path working without special nil-checks.
type NoopEmbedder struct {
	dims int // Number of dimensions in each zero vector.
}

// NewNoopEmbedder creates a NoopEmbedder with the given dimensionality.
func NewNoopEmbedder(dims int) *NoopEmbedder {
	return &NoopEmbedder{dims: dims}
}

// Embed returns a zero vector of the configured dimensionality.
func (e *NoopEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	return make([]float32, e.dims), nil
}

// EmbedBatch returns one zero vector per input text.
func (e *NoopEmbedder) EmbedBatch(_ context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i := range texts {
		result[i] = make([]float32, e.dims)
	}
	return result, nil
}

// Dimensions returns the configured vector dimensionality.
func (e *NoopEmbedder) Dimensions() int {
	return e.dims
}
