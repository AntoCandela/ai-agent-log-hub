package embed

import "context"

// NoopEmbedder returns zero vectors. Used for testing and when embedding is not configured.
type NoopEmbedder struct {
	dims int
}

// NewNoopEmbedder creates a NoopEmbedder with the given dimensionality.
func NewNoopEmbedder(dims int) *NoopEmbedder {
	return &NoopEmbedder{dims: dims}
}

func (e *NoopEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	return make([]float32, e.dims), nil
}

func (e *NoopEmbedder) EmbedBatch(_ context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i := range texts {
		result[i] = make([]float32, e.dims)
	}
	return result, nil
}

func (e *NoopEmbedder) Dimensions() int {
	return e.dims
}
