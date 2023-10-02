package model

import (
	"sort"

	v1 "k8s.io/api/core/v1"
)

type nodeInfos map[string][]LabelValue

func NewNodeInfos() *nodeInfos {
	newNodeInfos := make(nodeInfos)
	return &newNodeInfos
}

func (n *nodeInfos) WithNodes(nodes *v1.NodeList) *nodeInfos {
	for _, node := range nodes.Items {
		(*n)[node.Name] = []LabelValue{}
	}

	return n
}

func (n *nodeInfos) sortedKeys() []string {
	keys := make([]string, 0, len(*n))
	for k := range *n {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (n *nodeInfos) maxNodeNameLength() int {
	var max int
	for name := range *n {
		if len(name) > max {
			max = len(name)
		}
	}
	return max
}

func (n *nodeInfos) appendLabelValueTo(nodeName string, labelValue *LabelValue) {
	(*n)[nodeName] = append((*n)[nodeName], *labelValue)
}
