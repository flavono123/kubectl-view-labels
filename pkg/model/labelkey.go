package model

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/lithammer/fuzzysearch/fuzzy"
)

/* LabeKey */
type LabelKey struct {
	Name  string
	Style lipgloss.Style
}

func NewLabelKey() *LabelKey {
	return &LabelKey{}
}

func (k *LabelKey) WithName(name string) *LabelKey {
	k.Name = name
	color := hashToColorCode(hash(name))
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
	k.Style = style

	return k
}

// TODO: should be pull up to common pkg
func (k *LabelKey) Render() string {
	value := ""
	style := k.Style.Copy()

	if len(k.Name) > searchModelWidth {
		value = k.Name[:searchModelWidth-3] + "..."
	} else {
		value = k.Name
	}

	return style.Render(value)
}

/* LabeKeys */
func LabelKeyNames(keys []LabelKey) []string {
	var names []string
	for _, key := range keys {
		names = append(names, key.Name)
	}
	return names
}

func FuzzyFindLabelKeys(input string, keys []LabelKey) []LabelKey {
	var results []LabelKey
	for _, key := range keys {
		if fuzzy.Match(input, key.Name) {
			results = append(results, key)
		}
	}
	return results
}
