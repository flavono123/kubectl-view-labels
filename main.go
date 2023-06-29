package main

import (
	"context"
	"flag"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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

func (k *LabelKey) Render() string {
	return k.Style.Render(k.Name)
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

/* LabelValue */
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
	return v.Style.Render(v.Name)
}

/* NodeInfos */
type NodeInfos map[string][]LabelValue

func FilterNodeInfos(keys []LabelKey, infos NodeInfos) NodeInfos {
	FilteredNodeInfos := make(NodeInfos)
	labeKeyNames := LabelKeyNames(keys)
	for _, node := range Nodes.Items {
		for key := range node.Labels {
			if contains(labeKeyNames, key) {
				for _, key := range keys {
					labelValue := NewLabelValue().WithName(node.Labels[key.Name]).WithKey(key)
					FilteredNodeInfos[node.Name] = append(FilteredNodeInfos[node.Name], *labelValue)
				}
				break
			}
		}
	}

	return FilteredNodeInfos
}

func MaxNodeNameLength(infos NodeInfos) int {
	var max int
	for name := range infos {
		if len(name) > max {
			max = len(name)
		}
	}
	return max
}

/* Model */
type model struct {
	FilteredNodeInfos NodeInfos
	FilteredLabelKeys []LabelKey
	Paginator         paginator.Model
	TextInput         textinput.Model
}

/* Global */
var (
	Nodes     *v1.NodeList
	LabelKeys []LabelKey
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
	var labelKeys []LabelKey
	for _, node := range Nodes.Items {
		for key := range node.Labels {
			labelKey := NewLabelKey().WithName(key)
			labelKeys = append(labelKeys, *labelKey)
		}
	}
	uniqLabelKeys := uniqueKeys(labelKeys)
	sort.Slice(uniqLabelKeys, func(i, j int) bool {
		return uniqLabelKeys[i].Name < uniqLabelKeys[j].Name
	})
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
	nodeInfos := make(NodeInfos)
	for _, node := range Nodes.Items {
		nodeInfos[node.Name] = []LabelValue{}
	}

	return model{
		FilteredNodeInfos: nodeInfos,
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
	m.FilteredLabelKeys = FuzzyFindLabelKeys(m.TextInput.Value(), LabelKeys)
	m.FilteredNodeInfos = FilterNodeInfos(m.FilteredLabelKeys, m.FilteredNodeInfos)
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
		sb.WriteString(labelKey.Render() + "\n")
	}
	sb.WriteString("\n\n  " + m.Paginator.View())
	sb.WriteString("\n\n  ←/→ page • ctrl+c: quit\n")

	// Result view
	var rb strings.Builder

	rb.WriteString("Node list\n\n")

	max := MaxNodeNameLength(m.FilteredNodeInfos)
	for _, name := range sortedKeys(m.FilteredNodeInfos) {
		line := fmt.Sprintf("%-"+strconv.Itoa(max+2)+"s", name)
		for _, labelValue := range m.FilteredNodeInfos[name] {
			if len(line) > 150 {
				line += " ..."
				break
			}

			line += " "
			line = line + labelValue.Render()
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

func uniqueKeys(input []LabelKey) []LabelKey {
	seen := make(map[string]struct{})
	unique := []LabelKey{}

	for _, key := range input {
		if _, ok := seen[key.Name]; !ok {
			unique = append(unique, key)
			seen[key.Name] = struct{}{}
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

func sortedKeys(m NodeInfos) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
