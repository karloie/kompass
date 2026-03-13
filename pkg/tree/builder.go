package tree

import (
	"sort"

	"github.com/karloie/kompass/pkg/graph"
	kube "github.com/karloie/kompass/pkg/kube"
)

type ChildBuilder func(string, kube.Resource, map[string][]string, *treeBuildState, map[string]kube.Resource) []*kube.Tree

var childBuilders map[string]ChildBuilder

func init() {
	childBuilders = map[string]ChildBuilder{
		"cronjob":               buildCronJobChildren,
		"deployment":            buildWorkloadChildren,
		"statefulset":           buildWorkloadChildren,
		"daemonset":             buildWorkloadChildren,
		"job":                   buildJobChildren,
		"replicaset":            buildReplicaSetChildren,
		"service":               buildServiceChildren,
		"serviceaccount":        buildServiceAccountChildren,
		"pod":                   buildPodChildren,
		"endpoints":             buildEndpointsChildren,
		"endpointslice":         buildEndpointSliceChildren,
		"ciliumnetworkpolicy":   buildCiliumNetworkPolicyChildren,
		"persistentvolumeclaim": buildPersistentVolumeClaimChildren,
		"secret":                buildSecretChildren,
		"certificate":           buildCertificateChildren,
		"httproute":             buildHTTPRouteChildren,
	}
}

type ChildrenBuilder struct {
	children []*kube.Tree
}

func NewChildrenBuilder() *ChildrenBuilder {
	return &ChildrenBuilder{
		children: make([]*kube.Tree, 0),
	}
}

func (cb *ChildrenBuilder) Add(node *kube.Tree) *ChildrenBuilder {
	if node != nil {
		cb.children = append(cb.children, node)
	}
	return cb
}

func (cb *ChildrenBuilder) Extend(nodes []*kube.Tree) *ChildrenBuilder {
	cb.children = append(cb.children, nodes...)
	return cb
}

func (cb *ChildrenBuilder) Build() []*kube.Tree {
	sortChildren(cb.children)
	return cb.children
}

func sortChildren(children []*kube.Tree) {
	sort.Slice(children, func(i, j int) bool {
		if children[i].Type != children[j].Type {
			return children[i].Type < children[j].Type
		}
		nameI := ""
		nameJ := ""
		if name, ok := children[i].Meta["name"].(string); ok {
			nameI = name
		}
		if name, ok := children[j].Meta["name"].(string); ok {
			nameJ = name
		}
		return nameI < nameJ
	})
}

func BuildResponseTree(graphSet *kube.Graphs) *kube.Trees {
	if graphSet == nil {
		return nil
	}
	out := &kube.Trees{Nodes: graphSet.Nodes, Trees: make([]*kube.Tree, 0, len(graphSet.Graphs))}
	for i := range graphSet.Graphs {
		graphNodes := graphNodesForTree(graphSet)
		treeNode := BuildTree(graphSet.Graphs[i].ID, graphSet.Graphs[i].Edges, graphNodes)
		out.Trees = append(out.Trees, treeNode)
	}
	return out
}

func graphNodesForTree(graphSet *kube.Graphs) map[string]kube.Resource {
	nodeMap := make(map[string]kube.Resource)
	if graphSet != nil && len(graphSet.Nodes) > 0 {
		for key, node := range graphSet.Nodes {
			if node != nil {
				nodeMap[key] = *node
			}
		}
	}
	return nodeMap
}

func BuildTree(rootKey string, edges []kube.ResourceEdge, nodeMap map[string]kube.Resource) *kube.Tree {
	children := buildTreeAdjacency(edges, nodeMap)
	normalizeChildrenMap(children)

	state := newTreeBuildState()
	return buildTreeNode(rootKey, children, state, nodeMap)
}

func buildTreeNode(key string, children map[string][]string, state *treeBuildState, nodeMap map[string]kube.Resource) *kube.Tree {
	resource, exists := nodeMap[key]
	if !exists {
		return nil
	}

	state.MarkSeen(key)

	treeNode := NewTree(key, resource.Type, map[string]any{})

	if builder, hasBuilder := childBuilders[resource.Type]; hasBuilder {
		treeNode.Children = builder(key, resource, children, state, nodeMap)
		return treeNode
	}

	var leafChildrenTypes map[string]bool
	if proc, ok := graph.ResourceTypes[resource.Type]; ok && len(proc.LeafChildren) > 0 {
		leafChildrenTypes = make(map[string]bool)
		for _, leafType := range proc.LeafChildren {
			leafChildrenTypes[leafType] = true
		}
	}

	for _, childKey := range children[key] {
		if state.CanTraverse(childKey) {
			childResource, childExists := nodeMap[childKey]
			if !childExists {
				continue
			}

			if leafChildrenTypes != nil && leafChildrenTypes[childResource.Type] {
				leafNode := NewTree(childKey, childResource.Type, map[string]any{})
				treeNode.Children = append(treeNode.Children, leafNode)
				state.MarkSeen(childKey)
			} else {
				if childNode := buildTreeNode(childKey, children, state, nodeMap); childNode != nil {
					treeNode.Children = append(treeNode.Children, childNode)
				}
			}
		}
	}

	sortChildren(treeNode.Children)
	return treeNode
}

func NewTree(key, nodeType string, meta map[string]any) *kube.Tree {
	if meta == nil {
		meta = map[string]any{}
	}
	return &kube.Tree{
		Key:      key,
		Type:     nodeType,
		Meta:     meta,
		Children: []*kube.Tree{},
	}
}
