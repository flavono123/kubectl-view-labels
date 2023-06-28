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
	"github.com/muesli/termenv"

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
	Nodes             *v1.NodeList
	LabelKeys         []string
	FilteredLabelKeys []string
	Paginator         paginator.Model
	TextInput         textinput.Model
}

func initialModel() model {
	// Path to kubeconfig file
	kubeconfig := flag.String("kubeconfig", filepath.Join(homeDir(), ".kube", "config"), "(optional) absolute path to the kubeconfig file")

	// Create the client config
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	panicIfError(err)

	// Create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	panicIfError(err)

	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	panicIfError(err)

	// List all label key of nodes
	var labelKeys []string
	for _, node := range nodes.Items {
		for labelKey := range node.Labels {
			labelKeys = append(labelKeys, labelKey)
		}
	}
	uniqueLabelKeys := uniqueStrings(labelKeys)
	sort.Strings(uniqueLabelKeys)

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
	p.SetTotalPages(len(uniqueLabelKeys))

	return model{
		Nodes:             nodes,
		LabelKeys:         uniqueLabelKeys,
		FilteredLabelKeys: uniqueLabelKeys,
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
	m.FilteredLabelKeys = fuzzy.Find(m.TextInput.Value(), m.LabelKeys)
	m.Paginator.SetTotalPages(len(m.FilteredLabelKeys))

	return m, cmd
}

var colorProfile = termenv.ColorProfile()

func (m model) View() string {
	var b strings.Builder

	b.WriteString("Label list\n\n")
	b.WriteString(m.TextInput.View() + "\n\n")
	start, end := m.Paginator.GetSliceBounds(len(m.FilteredLabelKeys))
	for _, labelKey := range m.FilteredLabelKeys[start:end] {
		color := hashToColorCode(hash(labelKey))
		styledLabelKey := termenv.String(labelKey).Foreground(colorProfile.Color(color)).String()
		b.WriteString(styledLabelKey + "\n")
	}
	b.WriteString("\n\n  " + m.Paginator.View())
	b.WriteString("\n\n  ←/→ page • ctrl+c: quit\n")

	return b.String()
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
