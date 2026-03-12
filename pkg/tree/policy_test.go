package tree

import (
	"testing"

	"github.com/karloie/kompass/pkg/kube"
)

func TestFilterOwnedJobRoots_RemovesJobWhenCronJobRootExists(t *testing.T) {
	cronJobID := "cronjob/applikasjonsplattform/appwatch"
	jobID := "job/applikasjonsplattform/appwatch-29552340"

	resp := &kube.Graphs{
		Nodes: map[string]*kube.Resource{
			cronJobID: newRootNode(cronJobID, "cronjob", "appwatch", "applikasjonsplattform", nil),
			jobID:     newRootNode(jobID, "job", "appwatch-29552340", "applikasjonsplattform", []map[string]any{{"kind": "CronJob", "name": "appwatch"}}),
		},
		Graphs: []kube.Graph{
			{ID: cronJobID},
			{ID: jobID},
		},
	}

	FilterOwnedJobRoots(resp)

	if len(resp.Graphs) != 1 {
		t.Fatalf("expected 1 graph after filtering, got %d", len(resp.Graphs))
	}
	if resp.Graphs[0].ID != cronJobID {
		t.Fatalf("expected remaining root %q, got %q", cronJobID, resp.Graphs[0].ID)
	}
}

func TestFilterOwnedJobRoots_KeepsDetachedJobRoot(t *testing.T) {
	jobID := "job/applikasjonsplattform/manual-job"

	resp := &kube.Graphs{
		Nodes: map[string]*kube.Resource{
			jobID: newRootNode(jobID, "job", "manual-job", "applikasjonsplattform", nil),
		},
		Graphs: []kube.Graph{
			{ID: jobID},
		},
	}

	FilterOwnedJobRoots(resp)

	if len(resp.Graphs) != 1 {
		t.Fatalf("expected detached job root to remain, got %d graphs", len(resp.Graphs))
	}
	if resp.Graphs[0].ID != jobID {
		t.Fatalf("expected remaining root %q, got %q", jobID, resp.Graphs[0].ID)
	}
}

func newRootNode(id, typ, name, namespace string, owners []map[string]any) *kube.Resource {
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

	return &kube.Resource{
		Key:  id,
		Type: typ,
		Resource: map[string]any{
			"metadata": meta,
		},
	}
}
