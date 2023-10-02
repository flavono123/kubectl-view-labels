package model

import "github.com/charmbracelet/lipgloss"

type LabelValue struct {
	Name  string
	Key   LabelKey
	Style lipgloss.Style
}

func NewLabelValue() *LabelValue {
	return &LabelValue{}
}

func (v *LabelValue) WithName(name string) *LabelValue {
	v.Name = name

	return v
}

func (v *LabelValue) WithKey(key LabelKey) *LabelValue {
	v.Key = key
	v.Style = key.Style

	return v
}

func (v *LabelValue) Render() string {
	style := v.Style.Copy()

	if v.Name == "" {
		return style.Italic(true).Render("<None>")
	} else {
		return style.Render(v.Name)
	}
}
