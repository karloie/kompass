package tree

import (
	"testing"

	"github.com/karloie/kompass/pkg/kube"
)

func TestFilterOwnedJobRoots_RemovesJobWhenCronJobRootExists(t *testing.T) {
	cronJobID := "cronjob/applikasjonsplattform/appwatch"
	jobID := "job/applikasjonsplattform/appwatch-29552340"

	resp := &kube.Response{
		Nodes:      []kube.Resource{*newRootNode(cronJobID, "cronjob", "appwatch", "applikasjonsplattform", nil), *newRootNode(jobID, "job", "appwatch-29552340", "applikasjonsplattform", []map[string]any{{"kind": "CronJob", "name": "appwatch"}})},
		Components: []kube.Component{{ID: cronJobID, Root: cronJobID}, {ID: jobID, Root: jobID}},
	}

	FilterOwnedJobRoots(resp)

	if len(resp.Components) != 1 {
		t.Fatalf("expected 1 component after filtering, got %d", len(resp.Components))
	}
	if resp.Components[0].Root != cronJobID {
		t.Fatalf("expected remaining root %q, got %q", cronJobID, resp.Components[0].Root)
	}
}

func TestFilterOwnedJobRoots_KeepsDetachedJobRoot(t *testing.T) {
	jobID := "job/applikasjonsplattform/manual-job"

	resp := &kube.Response{
		Nodes:      []kube.Resource{*newRootNode(jobID, "job", "manual-job", "applikasjonsplattform", nil)},
		Components: []kube.Component{{ID: jobID, Root: jobID}},
	}

	FilterOwnedJobRoots(resp)

	if len(resp.Components) != 1 {
		t.Fatalf("expected detached job root to remain, got %d components", len(resp.Components))
	}
	if resp.Components[0].Root != jobID {
		t.Fatalf("expected remaining root %q, got %q", jobID, resp.Components[0].Root)
	}
}

func TestFilterOwnedSecretRoots_RemovesSecretWhenOwnerRootExists(t *testing.T) {
	replicaSetID := "replicaset/applikasjonsplattform/fiskeoye-ccfc7549"
	secretID := "secret/applikasjonsplattform/fiskeoye-secrets"

	resp := &kube.Response{
		Nodes:      []kube.Resource{*newRootNode(replicaSetID, "replicaset", "fiskeoye-ccfc7549", "applikasjonsplattform", nil), *newRootNode(secretID, "secret", "fiskeoye-secrets", "applikasjonsplattform", []map[string]any{{"kind": "ReplicaSet", "name": "fiskeoye-ccfc7549"}})},
		Components: []kube.Component{{ID: replicaSetID, Root: replicaSetID}, {ID: secretID, Root: secretID}},
	}

	FilterOwnedSecretRoots(resp)

	if len(resp.Components) != 1 {
		t.Fatalf("expected 1 component after filtering, got %d", len(resp.Components))
	}
	if resp.Components[0].Root != replicaSetID {
		t.Fatalf("expected remaining root %q, got %q", replicaSetID, resp.Components[0].Root)
	}
}

func TestFilterOwnedSecretRoots_KeepsDetachedSecretRoot(t *testing.T) {
	secretID := "secret/applikasjonsplattform/ad-explore-db-secret"

	resp := &kube.Response{
		Nodes:      []kube.Resource{*newRootNode(secretID, "secret", "ad-explore-db-secret", "applikasjonsplattform", nil)},
		Components: []kube.Component{{ID: secretID, Root: secretID}},
	}

	FilterOwnedSecretRoots(resp)

	if len(resp.Components) != 1 {
		t.Fatalf("expected detached secret root to remain, got %d components", len(resp.Components))
	}
	if resp.Components[0].Root != secretID {
		t.Fatalf("expected remaining root %q, got %q", secretID, resp.Components[0].Root)
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
