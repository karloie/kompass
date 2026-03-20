package tree

import (
	"sort"

	kube "github.com/karloie/kompass/pkg/kube"
)

type childBuilder func(string, kube.Resource, map[string][]string, *treeBuildState, map[string]kube.Resource) []*kube.Tree

var childBuilders map[string]childBuilder

func init() {
	childBuilders = map[string]childBuilder{
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

type childrenBuilder struct {
	children []*kube.Tree
}

func newChildrenBuilder() *childrenBuilder {
	return &childrenBuilder{
		children: make([]*kube.Tree, 0),
	}
}

func (cb *childrenBuilder) Add(node *kube.Tree) *childrenBuilder {
	if node != nil {
		cb.children = append(cb.children, node)
	}
	return cb
}

func (cb *childrenBuilder) Extend(nodes []*kube.Tree) *childrenBuilder {
	cb.children = append(cb.children, nodes...)
	return cb
}

func (cb *childrenBuilder) Build() []*kube.Tree {
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

// BuildTrees transforms a graph response into a hierarchical tree representation.
// Each connected component becomes a tree with parent-child relationships,
// enriched with metadata for display purposes.
func BuildTrees(graphSet *kube.Response) *kube.Response {
	if graphSet == nil {
		return nil
	}
	out := &kube.Response{Nodes: graphSet.Nodes, Trees: make([]kube.Tree, 0, len(graphSet.Components)), Metadata: graphSet.Metadata}
	for i := range graphSet.Components {
		graphNodes := graphNodesForTree(graphSet)
		treeNode := buildTreeInternal(graphSet.Components[i].Root, graphSet.Edges, graphNodes)
		if treeNode != nil {
			normalizePolicyPlacement(treeNode)
			out.Trees = append(out.Trees, *treeNode)
		}
	}
	// Enrich each tree node with structured metadata so web clients don't need
	// to run the ASCII-text rendering pass just to get display labels.
	nodeMap := out.NodeMap()
	for i := range out.Trees {
		enrichTreeMeta(&out.Trees[i], nodeMap)
	}
	return out
}

func graphNodesForTree(graphSet *kube.Response) map[string]kube.Resource {
	nodeMap := make(map[string]kube.Resource)
	if graphSet != nil && len(graphSet.Nodes) > 0 {
		for i := range graphSet.Nodes {
			node := graphSet.Nodes[i]
			nodeMap[node.Key] = node
		}
	}
	return nodeMap
}

func buildTreeInternal(rootKey string, edges []kube.ResourceEdge, nodeMap map[string]kube.Resource) *kube.Tree {
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

	treeNode := newTree(key, resource.Type, map[string]any{})

	if builder, hasBuilder := childBuilders[resource.Type]; hasBuilder {
		treeNode.Children = builder(key, resource, children, state, nodeMap)
		return treeNode
	}

	var leafChildrenTypes map[string]bool
	if meta, ok := kube.ResourceTypes[resource.Type]; ok && len(meta.NoRecurse) > 0 {
		leafChildrenTypes = make(map[string]bool)
		for _, leafType := range meta.NoRecurse {
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
				leafNode := newTree(childKey, childResource.Type, map[string]any{})
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

func newTree(key, nodeType string, meta map[string]any) *kube.Tree {
	if meta == nil {
		meta = map[string]any{}
	}
	return &kube.Tree{
		Key:      key,
		Type:     nodeType,
		Icon:     kube.GetResourceEmoji(nodeType),
		Meta:     meta,
		Children: []*kube.Tree{},
	}
}
