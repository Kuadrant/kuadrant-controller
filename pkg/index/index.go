package index

import (
	"strings"
	"sync"
)

const rootLabel string = "."

type TargetType int

const (
	GATEWAY_OVERRIDE = iota
	GATEWAY_DEFAULT
	ROUTE_DEFAULT
)

type IndexNode struct {
	label           string
	gatewayOverride *string
	gatewayDefault  *string
	routeDefault    *string
	children        map[string]*IndexNode
}

func (i *IndexNode) GetCreateAndDeleteObjects() {
	createObjects := []string{}
	starNode, present := i.children["*"]
	if present {
		if starNode.gatewayOverride != nil {
			createObjects = append(createObjects, *starNode.gatewayOverride)

			// remove rest of the nodes
			for label, node := range i.children {
				if label == "*" {
					continue
				}

			}
		}
	}
}

type IndexTree struct {
	mu   sync.RWMutex
	root *IndexNode
}

func NewIndex() *IndexTree {
	return &IndexTree{
		mu: sync.RWMutex{},
		root: &IndexNode{
			label:           rootLabel,
			children:        make(map[string]*IndexNode),
			gatewayOverride: nil,
			gatewayDefault:  nil,
			routeDefault:    nil,
		},
	}
}

// Get retrieves the node from the Index Tree
func (i *IndexTree) Get(key string) *IndexNode {
	i.mu.Lock()
	defer i.mu.Unlock()

	currPtr := i.root
	labels := traversableOrder(key)

	for _, label := range labels {
		child, present := currPtr.children[label]
		if !present {
			return nil
		}
		currPtr = child
	}

	return currPtr
}

func (i *IndexTree) GetParent(key string) *IndexNode {
	i.mu.Lock()
	defer i.mu.Unlock()

	currNode := i.root
	labels := traversableOrder(key)

	for idx, label := range labels {
		if idx == (len(labels) - 1) { // stop at the parent node
			break
		}
		child, present := currNode.children[label]
		if !present {
			return nil
		}
		currNode = child
	}
	return currNode
}

// Set tries to set the value in the Index and return true if insertion was successful.
func (i *IndexTree) Set(key, value string, targetType TargetType) bool {
	i.mu.Lock()
	defer i.mu.Unlock()

	currPtr := i.root
	labels := traversableOrder(key)

	for _, label := range labels {
		// if node is not present, create them.
		if _, present := currPtr.children[label]; !present {
			currPtr.children[label] = &IndexNode{
				label:           label,
				children:        make(map[string]*IndexNode),
				gatewayOverride: nil,
				gatewayDefault:  nil,
				routeDefault:    nil,
			}
		}
		currPtr = currPtr.children[label]
	}

	switch targetType {
	case GATEWAY_OVERRIDE:
		if currPtr.gatewayOverride != nil && *currPtr.gatewayOverride != value {
			return false
		}
		currPtr.gatewayOverride = &value
	case GATEWAY_DEFAULT:
		if currPtr.gatewayDefault != nil && *currPtr.gatewayDefault != value {
			return false
		}
		currPtr.gatewayDefault = &value
	default:
		if currPtr.routeDefault != nil && *currPtr.routeDefault != value {
			return false
		}
		currPtr.routeDefault = &value
	}

	return true
}

func traversableOrder(key string) []string {
	labels := strings.Split(key, ".")
	for i, j := 0, len(labels)-1; i < j; i, j = i+1, j-1 {
		labels[i], labels[j] = labels[j], labels[i]
	}
	return labels
}
