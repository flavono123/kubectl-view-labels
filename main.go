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
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	helpFlag := flag.Bool("help", false, "Print help message")
	flag.BoolVar(helpFlag, "h", false, "Print help message")

	flag.Usage = func() {
		printHelpMessage()
	}

	flag.Parse()

	if *helpFlag {
		printHelpMessage()
		os.Exit(0)
	}

	if len(os.Args) < 2 {
		printHelpMessage()
		os.Exit(1)
	}

	if os.Args[1] != "node" && os.Args[1] != "no" && os.Args[1] != "nodes" {
		printHelpMessage()
		os.Exit(1)
	}

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
	style := v.Style.Copy()

	if v.Name == "" {
		return style.Italic(true).Render("<None>")
	} else {
		return style.Render(v.Name)
	}
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

	Nodes = watchNodes(clientset)
	// Nodes, err = clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	// panicIfError(err)

	// List all label key of nodes
	updateLabelKeys()

	// Finder prompt
	ti := textinput.New()
	ti.Placeholder = "Search labels"
	ti.PromptStyle = searchPromptStyle
	ti.Focus()

	// Paginator
	p := paginator.New()
	p.Type = paginator.Dots
	p.PerPage = 10
	p.ActiveDot = activeDot
	p.InactiveDot = inactiveDot
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

const (
	// Layout
	searchModelWidth = 40
	resultModelWidth = 120
	commonHeight     = 20

	// Colors
	searchPromptColor = "#D4BFFF"
)

var (
	searchPromptStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(searchPromptColor))

	searcherModelStyle = lipgloss.NewStyle().
				Width(searchModelWidth).
				Height(commonHeight).
				MarginRight(2)

	resultModelStyle = lipgloss.NewStyle().
				Width(resultModelWidth).
				Height(commonHeight)

	helpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262")).Render
	activeDot   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "235", Dark: "252"}).Render("•")
	inactiveDot = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "250", Dark: "238"}).Render("•")
)

func (m model) View() string {
	start, end := m.Paginator.GetSliceBounds(len(m.FilteredLabelKeys))

	// Searcher view
	var sb strings.Builder
	sb.WriteString("LABELS\n\n")
	sb.WriteString(m.TextInput.View() + "\n\n")

	for _, labelKey := range m.FilteredLabelKeys[start:end] {
		sb.WriteString(labelKey.Render() + "\n")
	}
	sb.WriteString("\n\n  " + m.Paginator.View())
	fmt.Fprintln(&sb, helpStyle("\n\n  ←/→ page • ctrl+c: quit"))

	// Result view
	var rb strings.Builder

	rb.WriteString("NODES\n\n")

	max := MaxNodeNameLength(m.FilteredNodeInfos)
	for numOfNodes, name := range sortedKeys(m.FilteredNodeInfos) {
		if numOfNodes >= commonHeight-4 {
			rb.WriteString("..." + "\n")
			break
		}
		line := fmt.Sprintf("%-"+strconv.Itoa(max+2)+"s", name)
		for _, labelValue := range m.FilteredNodeInfos[name] {
			if len(line) > searchModelWidth+resultModelWidth-3 { // HACK: result model width doesn't make sense -_-
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

func printHelpMessage() {
	fmt.Println(`Fuzzy search with label keys for a resource.

Usage:
	view-labels <resource>

Available Resources:
	node(no, nodes): Nodes

Options:
	-h, --help:
		Print help message`)
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

func watchNodes(clientset *kubernetes.Clientset) *v1.NodeList {
	// Get initial list of nodes
	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}

	// Set up the watch
	watchlist := cache.NewListWatchFromClient(clientset.CoreV1().RESTClient(), "nodes", "", fields.Everything())

	_, controller := cache.NewInformer(
		watchlist,
		&v1.Node{},
		0, // No resync
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				node := obj.(*v1.Node)
				// fmt.Println("Node added:", node.Name)
				// Handle the added node
				Nodes.Items = append(Nodes.Items, *node)
				updateLabelKeys()
			},
			DeleteFunc: func(obj interface{}) {
				node := obj.(*v1.Node)
				// fmt.Println("Node deleted:", node.Name)
				// Handle the deleted node
				for i, n := range Nodes.Items {
					if n.Name == node.Name {
						Nodes.Items = append(Nodes.Items[:i], Nodes.Items[i+1:]...)
						break
					}
				}
				updateLabelKeys()
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				node := newObj.(*v1.Node)
				// fmt.Println("Node updated:", node.Name)
				// Handle the updated node
				for i, n := range Nodes.Items {
					if n.Name == node.Name {
						Nodes.Items[i] = *node
						break
					}
				}
				updateLabelKeys()
			},
		},
	)

	stop := make(chan struct{})
	go controller.Run(stop)
	// You might need to handle stopping this gracefully in your application

	return nodes
}

func updateLabelKeys() {
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
}
