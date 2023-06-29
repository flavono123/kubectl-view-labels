package main

import (
	"context"
	"flag"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/paginator"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/lithammer/fuzzysearch/fuzzy"

	tea "github.com/charmbracelet/bubbletea"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}

type model struct {
	FilteredNodes     map[string][]string
	FilteredLabelKeys []string
	Paginator         paginator.Model
	TextInput         textinput.Model
}

var (
	Nodes     *v1.NodeList
	LabelKeys []string
)

func initialModel() model {
	// Path to kubeconfig file
	kubeconfig := flag.String("kubeconfig", filepath.Join(homeDir(), ".kube", "config"), "(optional) absolute path to the kubeconfig file")

	// Create the client config
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	panicIfError(err)

	// Create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	panicIfError(err)

	Nodes, err = clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	panicIfError(err)

	// List all label key of nodes
	var labelKeys []string
	for _, node := range Nodes.Items {
		for labelKey := range node.Labels {
			labelKeys = append(labelKeys, labelKey)
		}
	}
	uniqLabelKeys := uniqueStrings(labelKeys)
	sort.Strings(uniqLabelKeys)
	LabelKeys = uniqLabelKeys

	// Finder prompt
	ti := textinput.New()
	ti.Placeholder = "Search labels"
	ti.Focus()

	// Paginator
	p := paginator.New()
	p.Type = paginator.Dots
	p.PerPage = 10
	p.ActiveDot = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "235", Dark: "252"}).Render("•")
	p.InactiveDot = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "250", Dark: "238"}).Render("•")
	p.SetTotalPages(len(LabelKeys))

	// Node names
	nodeInfos := make(map[string][]string)
	for _, node := range Nodes.Items {
		nodeInfos[node.Name] = []string{}
	}

	return model{
		FilteredNodes:     nodeInfos,
		FilteredLabelKeys: LabelKeys,
		Paginator:         p,
		TextInput:         ti,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "left", "right":
			m.Paginator, cmd = m.Paginator.Update(msg)
			return m, cmd
		}
	}
	m.TextInput, cmd = m.TextInput.Update(msg)
	m.FilteredLabelKeys = fuzzy.Find(m.TextInput.Value(), LabelKeys)
	// Filter nodes by label key
	filteredNodes := make(map[string][]string)
	for _, node := range Nodes.Items {
		for labelKey := range node.Labels {
			if contains(m.FilteredLabelKeys, labelKey) {
				for _, filteredLabelKey := range m.FilteredLabelKeys {
					color := hashToColorCode(hash(filteredLabelKey))
					styledLabelValue := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(node.Labels[filteredLabelKey])
					filteredNodes[node.Name] = append(filteredNodes[node.Name], styledLabelValue)
				}
				break
			}
		}
	}
	m.FilteredNodes = filteredNodes
	m.Paginator.SetTotalPages(len(m.FilteredLabelKeys))

	return m, cmd
}

var (
	searcherModelStyle = lipgloss.NewStyle().
				Width(60).
				Height(20).
				BorderStyle(lipgloss.NormalBorder()). // for Debugging
				BorderForeground(lipgloss.Color("69"))

	resultModelStyle = lipgloss.NewStyle().
				MaxWidth(120).
				Height(20).
				BorderStyle(lipgloss.NormalBorder()). // for Debugging
				BorderForeground(lipgloss.Color("96"))
)

func (m model) View() string {
	start, end := m.Paginator.GetSliceBounds(len(m.FilteredLabelKeys))

	// Searcher view
	var sb strings.Builder
	sb.WriteString("Label list\n\n")
	sb.WriteString(m.TextInput.View() + "\n\n")

	for _, labelKey := range m.FilteredLabelKeys[start:end] {
		color := hashToColorCode(hash(labelKey))
		styledLabelKey := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(labelKey)
		sb.WriteString(styledLabelKey + "\n")
	}
	sb.WriteString("\n\n  " + m.Paginator.View())
	sb.WriteString("\n\n  ←/→ page • ctrl+c: quit\n")

	// Result view
	var rb strings.Builder

	rb.WriteString("Node list\n\n")

	for _, name := range sortedKeys(m.FilteredNodes) {
		line := name + ":"
		for _, labelValue := range m.FilteredNodes[name] {
			if len(line) > 150 {
				line += " ..."
				break
			}

			line += " "
			line = line + labelValue
		}

		rb.WriteString(line + "\n")
	}

	searcherBox := searcherModelStyle.Render(sb.String())
	resultBox := resultModelStyle.Render(rb.String())

	return lipgloss.JoinHorizontal(lipgloss.Top, searcherBox, resultBox)
}

/* Helpers */

func panicIfError(err error) {
	if err != nil {
		panic(err.Error())
	}
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // Windows
}

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

func hashToColorCode(hash uint32) string {
	return fmt.Sprintf("#%06x", hash&0x00ffffff) // use the lower 24 bits of the hash
}

func uniqueStrings(input []string) []string {
	seen := make(map[string]struct{})
	unique := []string{}

	for _, str := range input {
		if _, ok := seen[str]; !ok {
			unique = append(unique, str)
			seen[str] = struct{}{}
		}
	}

	return unique
}

func contains(input []string, str string) bool {
	for _, s := range input {
		if s == str {
			return true
		}
	}
	return false
}

func sortedKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
