package main

import (
	"testing"

	"github.com/karloie/kompass/pkg/kube"
)

func TestFilterOwnedJobRoots_RemovesJobWhenCronJobRootExists(t *testing.T) {
	cronJobID := "cronjob/applikasjonsplattform/appwatch"
	jobID := "job/applikasjonsplattform/appwatch-29552340"

	resp := &kube.GraphResponse{
		Graphs: []kube.Graph{
			newGraph(cronJobID, "cronjob", "appwatch", "applikasjonsplattform", nil),
			newGraph(jobID, "job", "appwatch-29552340", "applikasjonsplattform", []map[string]any{
				{"kind": "CronJob", "name": "appwatch"},
			}),
		},
	}

	filterOwnedJobRoots(resp)

	if len(resp.Graphs) != 1 {
		t.Fatalf("expected 1 graph after filtering, got %d", len(resp.Graphs))
	}
	if resp.Graphs[0].ID != cronJobID {
		t.Fatalf("expected remaining root %q, got %q", cronJobID, resp.Graphs[0].ID)
	}
}

func TestFilterOwnedJobRoots_KeepsDetachedJobRoot(t *testing.T) {
	jobID := "job/applikasjonsplattform/manual-job"

	resp := &kube.GraphResponse{
		Graphs: []kube.Graph{
			newGraph(jobID, "job", "manual-job", "applikasjonsplattform", nil),
		},
	}

	filterOwnedJobRoots(resp)

	if len(resp.Graphs) != 1 {
		t.Fatalf("expected detached job root to remain, got %d graphs", len(resp.Graphs))
	}
	if resp.Graphs[0].ID != jobID {
		t.Fatalf("expected remaining root %q, got %q", jobID, resp.Graphs[0].ID)
	}
}

func TestGraphNodesForGraph_UsesResponseNodesWithNodeKeys(t *testing.T) {
	shared := &kube.Resource{Key: "service/default/api", Type: "service", Resource: map[string]any{"metadata": map[string]any{"name": "api", "namespace": "default"}}}
	podA := &kube.Resource{Key: "pod/default/a", Type: "pod", Resource: map[string]any{"metadata": map[string]any{"name": "a", "namespace": "default"}}}
	podB := &kube.Resource{Key: "pod/default/b", Type: "pod", Resource: map[string]any{"metadata": map[string]any{"name": "b", "namespace": "default"}}}

	resp := &kube.GraphResponse{
		Nodes: map[string]*kube.Resource{
			"pod/default/a":       podA,
			"pod/default/b":       podB,
			"service/default/api": shared,
		},
		Graphs: []kube.Graph{
			{ID: "pod/default/a", NodeKeys: []string{"pod/default/a", "service/default/api"}},
			{ID: "pod/default/b", NodeKeys: []string{"pod/default/b", "service/default/api"}},
		},
	}

	nodesA := graphNodesForGraph(resp, &resp.Graphs[0])
	nodesB := graphNodesForGraph(resp, &resp.Graphs[1])

	if len(nodesA) != 2 || len(nodesB) != 2 {
		t.Fatalf("expected 2 nodes per graph, got %d and %d", len(nodesA), len(nodesB))
	}
	if nodesA["service/default/api"] == nil || nodesB["service/default/api"] == nil {
		t.Fatalf("expected shared node to be resolved for both graphs")
	}
}

func TestGraphNodesForGraph_FallsBackToGraphNodes(t *testing.T) {
	pod := &kube.Resource{Key: "pod/default/a", Type: "pod"}
	resp := &kube.GraphResponse{Graphs: []kube.Graph{
		{ID: "pod/default/a", Nodes: map[string]*kube.Resource{"pod/default/a": pod}},
	}}

	nodes := graphNodesForGraph(resp, &resp.Graphs[0])
	if len(nodes) != 1 {
		t.Fatalf("expected fallback graph nodes map")
	}
}

func newGraph(id, typ, name, namespace string, owners []map[string]any) kube.Graph {
	meta := map[string]any{
		"name":      name,
		"namespace": namespace,
	}
	if owners != nil {
		ownerSlice := make([]any, 0, len(owners))
		for _, owner := range owners {
			ownerSlice = append(ownerSlice, owner)
		}
		meta["ownerReferences"] = ownerSlice
	}

	node := &kube.Resource{
		Key:  id,
		Type: typ,
		Resource: map[string]any{
			"metadata": meta,
		},
	}

	return kube.Graph{
		ID:    id,
		Nodes: map[string]*kube.Resource{id: node},
	}
}
