package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNewRatesRepository(t *testing.T) {
	repo := NewRatesRepository(nil, zap.NewNop())
	assert.NotNil(t, repo)
	assert.NotNil(t, repo.tracer)
	assert.NotNil(t, repo.logger)
}
