package model

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
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

/* NodeInfos */
type NodeInfos map[string][]LabelValue

/* Model */
type model struct {
	Nodes             *v1.NodeList
	filteredNodeInfos NodeInfos
	TotalLabelKeys    []LabelKey
	FilteredLabelKeys []LabelKey
	Paginator         paginator.Model
	TextInput         textinput.Model
}

func (m *model) filterNodeInfos(keys []LabelKey) {
	filteredNodeInfos := make(NodeInfos)
	labeKeyNames := LabelKeyNames(keys)
	for _, node := range m.Nodes.Items {
		for key := range node.Labels {
			if contains(labeKeyNames, key) {
				for _, key := range keys {
					labelValue := NewLabelValue().WithName(node.Labels[key.Name]).WithKey(key)
					filteredNodeInfos[node.Name] = append(filteredNodeInfos[node.Name], *labelValue)
				}
				break
			}
		}
	}

	m.filteredNodeInfos = filteredNodeInfos
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

func InitialModel() *model {
	// Path to kubeconfig file
	kubeconfig := flag.String("kubeconfig", filepath.Join(homeDir(), ".kube", "config"), "(optional) absolute path to the kubeconfig file")

	// Create the client config
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	panicIfError(err)

	// Create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	panicIfError(err)

	m := model{}

	m.Nodes = watchNodes(clientset, &m)
	m.updateTotalLabelKeys()

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
	p.SetTotalPages(len(m.TotalLabelKeys))

	// Node names
	nodeInfos := make(NodeInfos)
	for _, node := range m.Nodes.Items {
		nodeInfos[node.Name] = []LabelValue{}
	}

	m.filteredNodeInfos = nodeInfos
	m.FilteredLabelKeys = m.TotalLabelKeys
	m.Paginator = p
	m.TextInput = ti

	return &m
}

func (m *model) Init() tea.Cmd {
	return nil
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
	m.FilteredLabelKeys = FuzzyFindLabelKeys(m.TextInput.Value(), m.TotalLabelKeys)
	m.filterNodeInfos(m.FilteredLabelKeys)
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

	max := MaxNodeNameLength(m.filteredNodeInfos)
	for numOfNodes, name := range sortedKeys(m.filteredNodeInfos) {
		if numOfNodes >= commonHeight-4 {
			rb.WriteString("..." + "\n")
			break
		}
		line := fmt.Sprintf("%-"+strconv.Itoa(max+2)+"s", name)
		for _, labelValue := range m.filteredNodeInfos[name] {
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

func watchNodes(clientset *kubernetes.Clientset, m *model) *v1.NodeList {
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
				m.Nodes.Items = append(m.Nodes.Items, *node)
				m.updateTotalLabelKeys()
			},
			DeleteFunc: func(obj interface{}) {
				node := obj.(*v1.Node)
				for i, n := range m.Nodes.Items {
					if n.Name == node.Name {
						m.Nodes.Items = append(m.Nodes.Items[:i], m.Nodes.Items[i+1:]...)
						break
					}
				}
				m.updateTotalLabelKeys()
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				node := newObj.(*v1.Node)
				for i, n := range m.Nodes.Items {
					if n.Name == node.Name {
						m.Nodes.Items[i] = *node
						break
					}
				}
				m.updateTotalLabelKeys()
			},
		},
	)

	stop := make(chan struct{})
	go controller.Run(stop)
	// You might need to handle stopping this gracefully in your application

	return nodes
}

func (m *model) updateTotalLabelKeys() {
	var labelKeys []LabelKey
	for _, node := range m.Nodes.Items {
		for key := range node.Labels {
			labelKey := NewLabelKey().WithName(key)
			labelKeys = append(labelKeys, *labelKey)
		}
	}

	// uniq
	labelKeys = uniqueKeys(labelKeys)
	// sort
	sort.Slice(labelKeys, func(i, j int) bool {
		return labelKeys[i].Name < labelKeys[j].Name
	})

	m.TotalLabelKeys = labelKeys
}

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
