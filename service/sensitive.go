package service

import (
	"errors"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting"
)

func CheckSensitiveMessages(messages []dto.Message) ([]string, error) {
	if len(messages) == 0 {
		return nil, nil
	}

	for _, message := range messages {
		arrayContent := message.ParseContent()
		for _, m := range arrayContent {
			if m.Type == "image_url" {
				// TODO: check image url
				continue
			}
			// 检查 text 是否为空
			if m.Text == "" {
				continue
			}
			if ok, words := SensitiveWordContains(m.Text); ok {
				return words, errors.New("sensitive words detected")
			}
		}
	}
	return nil, nil
}

func CheckSensitiveText(text string) (bool, []string) {
	return SensitiveWordContains(text)
}

// SensitiveWordContains 是否包含敏感词，返回是否包含敏感词和敏感词列表
func SensitiveWordContains(text string) (bool, []string) {
	matches := findSensitiveWordMatches(text, true)
	if len(matches) == 0 {
		return false, nil
	}
	return true, []string{matches[0].word}
}

// SensitiveWordReplace 敏感词替换，返回是否包含敏感词和替换后的文本
func SensitiveWordReplace(text string, returnImmediately bool) (bool, []string, string) {
	matches := findSensitiveWordMatches(text, returnImmediately)
	if len(matches) == 0 {
		return false, nil, text
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].start == matches[j].start {
			return matches[i].end > matches[j].end
		}
		return matches[i].start < matches[j].start
	})

	textRunes := []rune(text)
	words := make([]string, 0, len(matches))
	var builder strings.Builder
	builder.Grow(len(text))
	lastPos := 0
	for _, match := range matches {
		if match.start < lastPos {
			continue
		}
		builder.WriteString(string(textRunes[lastPos:match.start]))
		builder.WriteString("**###**")
		lastPos = match.end
		words = append(words, match.word)
	}
	builder.WriteString(string(textRunes[lastPos:]))
	return true, words, builder.String()
}

type sensitiveWordMatch struct {
	start int
	end   int
	word  string
}

func findSensitiveWordMatches(text string, returnImmediately bool) []sensitiveWordMatch {
	if len(setting.SensitiveWords) == 0 || text == "" {
		return nil
	}

	checkRunes := []rune(strings.ToLower(text))
	machine := getOrBuildAC(setting.SensitiveWords)
	if machine == nil {
		return nil
	}

	hits := machine.MultiPatternSearch(checkRunes, false)
	matches := make([]sensitiveWordMatch, 0, len(hits))
	for _, hit := range hits {
		end := hit.Pos + len(hit.Word)
		if !hasSensitiveWordBoundary(checkRunes, hit.Word, hit.Pos, end) {
			continue
		}
		matches = append(matches, sensitiveWordMatch{
			start: hit.Pos,
			end:   end,
			word:  string(hit.Word),
		})
		if returnImmediately {
			return matches
		}
	}
	return matches
}

func hasSensitiveWordBoundary(text, word []rune, start, end int) bool {
	if len(word) == 0 {
		return false
	}
	if isASCIIWordRune(word[0]) && start > 0 && isASCIIWordRune(text[start-1]) {
		return false
	}
	if isASCIIWordRune(word[len(word)-1]) && end < len(text) && isASCIIWordRune(text[end]) {
		return false
	}
	return true
}

func isASCIIWordRune(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_'
}
