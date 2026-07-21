package service

import (
	"testing"

	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSensitiveWordContainsUsesBoundariesForASCIIWords(t *testing.T) {
	originalWords := setting.SensitiveWords
	setting.SensitiveWords = []string{"EXP", "IDA", "Reverse Engineering", "逆向"}
	t.Cleanup(func() { setting.SensitiveWords = originalWords })

	tests := []struct {
		name     string
		text     string
		contains bool
		word     string
	}{
		{name: "short acronym does not match word prefix", text: "explicitly requested", contains: false},
		{name: "tool name does not match word infix", text: "validation passed", contains: false},
		{name: "standalone acronym matches", text: "EXP analysis", contains: true, word: "exp"},
		{name: "standalone tool name matches", text: "Open in IDA Pro", contains: true, word: "ida"},
		{name: "phrase matches case insensitively", text: "reverse engineering report", contains: true, word: "reverse engineering"},
		{name: "Chinese term retains substring matching", text: "这是逆向工程报告", contains: true, word: "逆向"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			contains, words := SensitiveWordContains(test.text)
			assert.Equal(t, test.contains, contains)
			if !test.contains {
				assert.Empty(t, words)
				return
			}
			require.Len(t, words, 1)
			assert.Equal(t, test.word, words[0])
		})
	}
}

func TestSensitiveWordReplaceUsesRuneOffsetsAndSkipsASCIIFalsePositives(t *testing.T) {
	originalWords := setting.SensitiveWords
	setting.SensitiveWords = []string{"EXP", "逆向"}
	t.Cleanup(func() { setting.SensitiveWords = originalWords })

	contains, words, replaced := SensitiveWordReplace(
		"explicitly discusses 逆向工程 and EXP.",
		false,
	)

	require.True(t, contains)
	assert.Equal(t, []string{"逆向", "exp"}, words)
	assert.Equal(t, "explicitly discusses **###**工程 and **###**.", replaced)
}
