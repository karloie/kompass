package tree

import (
	"testing"

	kube "github.com/karloie/kompass/pkg/kube"
)

func TestBuildCronJobChildren_ExpandsFocusJobAndKeepsPodLeavesForHistory(t *testing.T) {
	cronJobKey := "cronjob/applikasjonsplattform/fiskeoye-repo-update"
	olderJobKey := "job/applikasjonsplattform/fiskeoye-repo-update-29552220"
	activeJobKey := "job/applikasjonsplattform/fiskeoye-repo-update-29555460"
	olderPodKey := "pod/applikasjonsplattform/fiskeoye-repo-update-29552220-nz92p"
	activePodKey := "pod/applikasjonsplattform/fiskeoye-repo-update-29555460-fzd8m"

	nodeMap := map[string]kube.Resource{
		cronJobKey: {
			Key:  cronJobKey,
			Type: "cronjob",
			Resource: map[string]any{
				"metadata": map[string]any{"name": "fiskeoye-repo-update", "namespace": "applikasjonsplattform"},
			},
		},
		olderJobKey: {
			Key:  olderJobKey,
			Type: "job",
			Resource: map[string]any{
				"metadata": map[string]any{"name": "fiskeoye-repo-update-29552220", "namespace": "applikasjonsplattform", "creationTimestamp": "2026-03-12T08:00:00Z"},
				"status":   map[string]any{"active": float64(0), "startTime": "2026-03-12T08:00:10Z"},
			},
		},
		activeJobKey: {
			Key:  activeJobKey,
			Type: "job",
			Resource: map[string]any{
				"metadata": map[string]any{"name": "fiskeoye-repo-update-29555460", "namespace": "applikasjonsplattform", "creationTimestamp": "2026-03-12T09:00:00Z"},
				"status":   map[string]any{"active": float64(1), "startTime": "2026-03-12T09:00:10Z"},
			},
		},
		olderPodKey: {
			Key:  olderPodKey,
			Type: "pod",
			Resource: map[string]any{
				"metadata": map[string]any{"name": "fiskeoye-repo-update-29552220-nz92p", "namespace": "applikasjonsplattform"},
				"status":   map[string]any{"phase": "Succeeded", "podIP": "10.244.9.131"},
				"spec":     map[string]any{"nodeName": "test-01-worker-055ceed2"},
			},
		},
		activePodKey: {
			Key:  activePodKey,
			Type: "pod",
			Resource: map[string]any{
				"metadata": map[string]any{"name": "fiskeoye-repo-update-29555460-fzd8m", "namespace": "applikasjonsplattform"},
				"status":   map[string]any{"phase": "Running", "podIP": "10.244.9.25"},
				"spec": map[string]any{
					"nodeName": "test-01-worker-055ceed2",
					"containers": []any{
						map[string]any{"name": "kubectl-exec", "image": "bitnami/kubectl:latest"},
					},
				},
			},
		},
	}

	graphChildren := map[string][]string{
		cronJobKey:  {olderJobKey, activeJobKey},
		olderJobKey: {cronJobKey, olderPodKey},
		activeJobKey: {
			cronJobKey,
			activePodKey,
		},
		olderPodKey:  {olderJobKey},
		activePodKey: {activeJobKey},
	}

	children := buildCronJobChildren(cronJobKey, nodeMap[cronJobKey], graphChildren, newTreeBuildState(), nodeMap)

	if len(children) != 2 {
		t.Fatalf("expected 2 jobs under cronjob, got %d", len(children))
	}

	var olderJobNode *kube.Tree
	var activeJobNode *kube.Tree
	for _, child := range children {
		switch child.Key {
		case olderJobKey:
			olderJobNode = child
		case activeJobKey:
			activeJobNode = child
		}
	}

	if olderJobNode == nil || activeJobNode == nil {
		t.Fatalf("expected both older and active job nodes")
	}

	if len(olderJobNode.Children) != 1 || olderJobNode.Children[0].Key != olderPodKey {
		t.Fatalf("expected older job to keep only one pod leaf")
	}
	if len(olderJobNode.Children[0].Children) != 0 {
		t.Fatalf("expected older pod to be a leaf (no subtree)")
	}
	if name, ok := olderJobNode.Children[0].Meta["name"].(string); !ok || name == "" {
		t.Fatalf("expected older pod leaf to include name metadata")
	}

	if len(activeJobNode.Children) != 1 || activeJobNode.Children[0].Key != activePodKey {
		t.Fatalf("expected active job to include active pod")
	}
	if len(activeJobNode.Children[0].Children) == 0 {
		t.Fatalf("expected active pod to keep expanded subtree")
	}
}

func TestBuildReplicaSetChildren_DeploymentOwned_ExpandsAllPods(t *testing.T) {
	rsKey := "replicaset/petshop/petshop-frontend-girls-598696998b"
	podAKey := "pod/petshop/petshop-frontend-girls-5cb9cd8b74-pqhk9"
	podBKey := "pod/petshop/petshop-frontend-girls-598696998b-v58bh"

	rs := kube.Resource{
		Key:  rsKey,
		Type: "replicaset",
		Resource: map[string]any{
			"metadata": map[string]any{
				"name":      "petshop-frontend-girls-598696998b",
				"namespace": "petshop",
				"ownerReferences": []any{
					map[string]any{"kind": "Deployment", "name": "petshop-frontend-girls"},
				},
			},
			"spec": map[string]any{},
		},
	}

	nodeMap := map[string]kube.Resource{
		rsKey: rs,
		podAKey: {
			Key:  podAKey,
			Type: "pod",
			Resource: map[string]any{
				"metadata": map[string]any{"name": "petshop-frontend-girls-5cb9cd8b74-pqhk9", "namespace": "petshop", "creationTimestamp": "2026-03-12T08:00:00Z"},
				"status": map[string]any{
					"phase":     "Running",
					"podIP":     "10.244.9.240",
					"startTime": "2026-03-12T08:00:10Z",
				},
				"spec": map[string]any{
					"nodeName":   "psb-01-worker-055ceed2",
					"containers": []any{map[string]any{"name": "app", "image": "petshop/petshop-frontend-girls:main"}},
				},
			},
		},
		podBKey: {
			Key:  podBKey,
			Type: "pod",
			Resource: map[string]any{
				"metadata": map[string]any{"name": "petshop-frontend-girls-598696998b-v58bh", "namespace": "petshop", "creationTimestamp": "2026-03-12T09:00:00Z"},
				"status": map[string]any{
					"phase":     "Running",
					"podIP":     "10.244.9.250",
					"startTime": "2026-03-12T09:00:10Z",
				},
				"spec": map[string]any{
					"containers": []any{map[string]any{"name": "app", "image": "petshop/petshop-frontend-girls:main"}},
				},
			},
		},
	}

	graphChildren := map[string][]string{
		rsKey:   {podAKey, podBKey},
		podAKey: {rsKey},
		podBKey: {rsKey},
	}

	children := buildReplicaSetChildren(rsKey, rs, graphChildren, newTreeBuildState(), nodeMap)

	if len(children) != 2 {
		t.Fatalf("expected 2 pod nodes under replicaset, got %d", len(children))
	}

	expandedCount := 0
	for _, child := range children {
		if child.Type != "pod" {
			continue
		}
		if len(child.Children) > 0 {
			expandedCount++
		} else {
			t.Fatalf("expected deployment-owned replicaset pods to be expanded")
		}
	}

	if expandedCount != 2 {
		t.Fatalf("expected all pods to be expanded, got %d expanded pods", expandedCount)
	}
}
