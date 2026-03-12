package id_test

import (
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"mcfg/internal/id"
)

func TestGenerate_ReturnsValidULID(t *testing.T) {
	value, err := id.ULIDGenerator{}.New()
	require.NoError(t, err)
	_, err = ulid.ParseStrict(value)
	require.NoError(t, err)
}

func TestGenerate_Unique(t *testing.T) {
	seen := map[string]struct{}{}
	gen := id.ULIDGenerator{}

	for range 1000 {
		value, err := gen.New()
		require.NoError(t, err)
		_, exists := seen[value]
		require.False(t, exists)
		seen[value] = struct{}{}
	}
}

func TestMatchByPrefix_ExactMatch(t *testing.T) {
	ids := []string{"01HQXBF7M6SJHMR6G32P5D1K7Y"}
	match, err := id.MatchByPrefix(ids[0], ids)
	require.NoError(t, err)
	require.Equal(t, ids[0], match)
}

func TestMatchByPrefix_8CharPrefix(t *testing.T) {
	ids := []string{"01HQXBF7M6SJHMR6G32P5D1K7Y", "01HQXBG84ESB7XJQ9WAAYH54AM"}
	match, err := id.MatchByPrefix("01HQXBF7", ids)
	require.NoError(t, err)
	require.Equal(t, ids[0], match)
}

func TestMatchByPrefix_Ambiguous(t *testing.T) {
	ids := []string{"01HQXBF7M6SJHMR6G32P5D1K7Y", "01HQXBF7ZZZZZZZZZZZZZZZZZZ"}
	_, err := id.MatchByPrefix("01HQXBF7", ids)
	require.Error(t, err)
}

func TestMatchByPrefix_TooShort(t *testing.T) {
	_, err := id.MatchByPrefix("short", []string{"01HQXBF7M6SJHMR6G32P5D1K7Y"})
	require.Error(t, err)
}
