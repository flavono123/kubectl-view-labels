package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/flavono123/kubectl-view-labels/pkg/model"
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

	m := model.InitialModel()
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}

/* Helpers */

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
