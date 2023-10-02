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
	"github.com/lithammer/fuzzysearch/fuzzy"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

/* Model */
type model struct {
	nodes             *v1.NodeList
	filteredNodeInfos *nodeInfos
	totalLabelKeys    []LabelKey
	filteredLabelKeys []LabelKey
	paginator         paginator.Model
	textInput         textinput.Model
}

func (m *model) filterNodeInfos() {
	filteredNodeInfos := NewNodeInfos()
	labeKeyNames := LabelKeyNames(m.filteredLabelKeys)
	for _, node := range m.nodes.Items {
		for key := range node.Labels {
			if contains(labeKeyNames, key) {
				for _, key := range m.filteredLabelKeys {
					labelValue := NewLabelValue().WithName(node.Labels[key.Name]).WithKey(key)
					filteredNodeInfos.appendLabelValueTo(node.Name, labelValue)
				}
				break
			}
		}
	}

	m.filteredNodeInfos = filteredNodeInfos
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

	m.nodes = watchNodes(clientset, &m)
	m.updateTotalLabelKeys()

	// Finder prompt
	ti := textinput.New()
	ti.Placeholder = "Search labels"
	ti.PromptStyle = searchPromptStyle
	ti.Focus()

	// paginator
	p := paginator.New()
	p.Type = paginator.Dots
	p.PerPage = 10
	p.ActiveDot = activeDot
	p.InactiveDot = inactiveDot
	p.SetTotalPages(len(m.totalLabelKeys))

	// Node names
	nodeInfos := NewNodeInfos().WithNodes(m.nodes)

	m.filteredNodeInfos = nodeInfos
	m.filteredLabelKeys = m.totalLabelKeys
	m.paginator = p
	m.textInput = ti

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
			m.paginator, cmd = m.paginator.Update(msg)
			return m, cmd
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	m.fuzzyFindLabelKeys(m.textInput.Value())
	m.filterNodeInfos()
	m.paginator.SetTotalPages(len(m.filteredLabelKeys))

	return m, cmd
}

func (m *model) fuzzyFindLabelKeys(input string) {
	var results []LabelKey
	for _, key := range m.totalLabelKeys {
		if fuzzy.Match(input, key.Name) {
			results = append(results, key)
		}
	}

	m.filteredLabelKeys = results
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
	start, end := m.paginator.GetSliceBounds(len(m.filteredLabelKeys))

	// Searcher view
	var sb strings.Builder
	sb.WriteString("LABELS\n\n")
	sb.WriteString(m.textInput.View() + "\n\n")

	for _, labelKey := range m.filteredLabelKeys[start:end] {
		sb.WriteString(labelKey.Render() + "\n")
	}
	sb.WriteString("\n\n  " + m.paginator.View())
	fmt.Fprintln(&sb, helpStyle("\n\n  ←/→ page • ctrl+c: quit"))

	// Result view
	var rb strings.Builder

	rb.WriteString("NODES\n\n")

	max := m.filteredNodeInfos.maxNodeNameLength()
	for numOfNodes, name := range m.filteredNodeInfos.sortedKeys() {
		if numOfNodes >= commonHeight-4 {
			rb.WriteString("..." + "\n")
			break
		}
		line := fmt.Sprintf("%-"+strconv.Itoa(max+2)+"s", name)
		for _, labelValue := range (*m.filteredNodeInfos)[name] {
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
				m.nodes.Items = append(m.nodes.Items, *node)
				m.updateTotalLabelKeys()
			},
			DeleteFunc: func(obj interface{}) {
				node := obj.(*v1.Node)
				for i, n := range m.nodes.Items {
					if n.Name == node.Name {
						m.nodes.Items = append(m.nodes.Items[:i], m.nodes.Items[i+1:]...)
						break
					}
				}
				m.updateTotalLabelKeys()
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				node := newObj.(*v1.Node)
				for i, n := range m.nodes.Items {
					if n.Name == node.Name {
						m.nodes.Items[i] = *node
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
	for _, node := range m.nodes.Items {
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

	m.totalLabelKeys = labelKeys
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
