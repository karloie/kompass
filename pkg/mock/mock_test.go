package mock

import (
	"strings"
	"testing"

	"github.com/karloie/kompass/pkg/graph"
	kube "github.com/karloie/kompass/pkg/kube"
	"github.com/karloie/kompass/pkg/tree"
)

func isWorkloadType(resourceType string) bool {
	switch resourceType {
	case "deployment", "replicaset", "statefulset", "daemonset", "job", "cronjob", "pod":
		return true
	default:
		return false
	}
}

func isCrossNamespaceType(resourceType string) bool {
	return resourceType == "certificate" || resourceType == "gateway" || resourceType == "clusterissuer"
}

func TestMockOutputStructureAndMetadata(t *testing.T) {

	client := kube.NewMockClient(GenerateMock())
	client.SetNamespace("petshop")

	resp, err := graph.InferGraphs(
		client,
		kube.Request{},
	)
	if err != nil {
		t.Fatalf("InferGraphs failed: %v", err)
	}
	treeResp := tree.BuildResponseTree(resp)

	t.Run("GraphOrdering", func(t *testing.T) {
		if len(resp.Components) == 0 {
			t.Fatal("Expected at least one component")
		}

		firstWorkloadIdx := -1
		firstStandaloneIdx := -1
		firstCrossNamespaceIdx := -1
		lastWorkloadIdx := -1
		lastStandaloneIdx := -1

		for idx, component := range resp.Components {
			parts := strings.Split(component.Root, "/")
			if len(parts) < 1 {
				continue
			}
			resourceType := parts[0]

			if isWorkloadType(resourceType) {
				if firstWorkloadIdx == -1 {
					firstWorkloadIdx = idx
				}
				lastWorkloadIdx = idx
			} else if isCrossNamespaceType(resourceType) {
				if firstCrossNamespaceIdx == -1 {
					firstCrossNamespaceIdx = idx
				}
			} else {

				if firstStandaloneIdx == -1 {
					firstStandaloneIdx = idx
				}
				lastStandaloneIdx = idx
			}
		}

		if firstWorkloadIdx != -1 && firstStandaloneIdx != -1 {
			if lastWorkloadIdx >= firstStandaloneIdx {
				t.Errorf("Workload graphs should all appear before standalone resources. Last workload at %d, first standalone at %d",
					lastWorkloadIdx, firstStandaloneIdx)
			}
		}

		if firstStandaloneIdx != -1 && firstCrossNamespaceIdx != -1 {
			if lastStandaloneIdx >= firstCrossNamespaceIdx {
				t.Errorf("Standalone resources should all appear before cross-namespace resources. Last standalone at %d, first cross-namespace at %d",
					lastStandaloneIdx, firstCrossNamespaceIdx)
			}
		}
	})

	t.Run("ResourceCounts", func(t *testing.T) {
		workloadGraphs := 0
		standaloneCNPs := 0
		standaloneConfigMaps := 0
		standaloneSecrets := 0
		standaloneServiceAccounts := 0
		crossNamespaceGraphs := 0

		for _, component := range resp.Components {
			parts := strings.Split(component.Root, "/")
			if len(parts) < 3 {
				continue
			}
			resourceType := parts[0]
			namespace := parts[1]
			resourceName := parts[2]

			if isWorkloadType(resourceType) {
				workloadGraphs++
			} else if isCrossNamespaceType(resourceType) {
				crossNamespaceGraphs++
			} else if namespace == "petshop" {

				switch resourceType {
				case "ciliumnetworkpolicy":
					if strings.HasPrefix(resourceName, "allow-") {
						standaloneCNPs++
					}
				case "configmap":
					if resourceName == "kube-root-ca.crt" {
						standaloneConfigMaps++
					}
				case "secret":
					if resourceName == "docker-registry-credentials" || resourceName == "tls-wildcard-cert" {
						standaloneSecrets++
					}
				case "serviceaccount":
					if resourceName == "default" {
						standaloneServiceAccounts++
					}
				}
			}
		}

		expectedWorkloads := 9
		if workloadGraphs != expectedWorkloads {
			t.Errorf("Expected %d workload graphs, got %d", expectedWorkloads, workloadGraphs)
		}

		expectedStandaloneCNPs := 3
		if standaloneCNPs != expectedStandaloneCNPs {
			t.Errorf("Expected %d standalone CiliumNetworkPolicies (allow-*), got %d", expectedStandaloneCNPs, standaloneCNPs)
		}

		expectedStandaloneConfigMaps := 1
		if standaloneConfigMaps != expectedStandaloneConfigMaps {
			t.Errorf("Expected %d standalone ConfigMaps, got %d", expectedStandaloneConfigMaps, standaloneConfigMaps)
		}

		expectedStandaloneSecrets := 1
		if standaloneSecrets != expectedStandaloneSecrets {
			t.Errorf("Expected %d standalone Secrets, got %d", expectedStandaloneSecrets, standaloneSecrets)
		}

		expectedStandaloneSAs := 1
		if standaloneServiceAccounts != expectedStandaloneSAs {
			t.Errorf("Expected %d standalone ServiceAccounts, got %d", expectedStandaloneSAs, standaloneServiceAccounts)
		}

		if crossNamespaceGraphs == 0 {
			t.Error("Expected at least one cross-namespace graph (gateway/certificate)")
		}
	})

	t.Run("RenderedMetadata", func(t *testing.T) {

		var rendered strings.Builder
		nodeMap := treeResp.NodeMap()
		for i := range treeResp.Trees {
			rendered.WriteString(tree.RenderTree(&treeResp.Trees[i], nodeMap, true))
			rendered.WriteString("\n")
		}
		output := rendered.String()

		if !strings.Contains(output, "deployment ") {
			t.Error("Expected to find deployment nodes in output")
		}
		if strings.Contains(output, "deployment ") {

			if !strings.Contains(output, "replicas=") {
				t.Error("Expected deployment to show 'replicas=' metadata")
			}
			if !strings.Contains(output, "strategy=") {
				t.Error("Expected deployment to show 'strategy=' metadata")
			}
		}

		if strings.Contains(output, "service ") {
			if !strings.Contains(output, "ports=") {
				t.Error("Expected service to show 'ports=' metadata")
			}
			if !strings.Contains(output, "type=") {
				t.Error("Expected service to show 'type=' metadata")
			}
		}

		if strings.Contains(output, "configmap ") {
			if !strings.Contains(output, "keys=") {
				t.Error("Expected configmap to show 'keys=' metadata")
			}
		}

		if strings.Contains(output, "ciliumnetworkpolicy ") {

			hasEgressOrIngress := strings.Contains(output, "cnp-egress ") || strings.Contains(output, "cnp-ingress ")
			if !hasEgressOrIngress {
				t.Error("Expected CiliumNetworkPolicy to show egress or ingress rules")
			}
		}

		if strings.Contains(output, "secret ") {
			if !strings.Contains(output, "type=") {
				t.Error("Expected secret to show 'type=' metadata")
			}

			if !strings.Contains(output, "keys=") {
				t.Error("Expected secret to show 'keys=' metadata")
			}
			if strings.Contains(output, "secretkeys:") {
				t.Error("Expected no separate secretkeys node (keys should be in secret metadata)")
			}
		}

		if strings.Contains(output, "pod ") {
			if !strings.Contains(output, "pod ") || !strings.Contains(output, " [RUNNING]") {
				t.Error("Expected pod to show phase as status in square brackets")
			}
		}

		if strings.Contains(output, "replicaset petshop-frontend-girls-598696998b") {
			podHasContainer := func(podName string) bool {
				podMarker := "pod " + podName
				idx := strings.Index(output, podMarker)
				if idx == -1 {
					return false
				}

				nextPodIdx := strings.Index(output[idx+len(podMarker):], "pod ")
				nextServiceIdx := strings.Index(output[idx+len(podMarker):], "service ")
				end := len(output)
				if nextPodIdx >= 0 {
					end = idx + len(podMarker) + nextPodIdx
				}
				if nextServiceIdx >= 0 {
					candidate := idx + len(podMarker) + nextServiceIdx
					if candidate < end {
						end = candidate
					}
				}

				segment := output[idx:end]
				return strings.Contains(segment, "container app")
			}

			xHasContainer := podHasContainer("petshop-frontend-girls-598696998b-tr5ft")
			yHasContainer := podHasContainer("petshop-frontend-girls-598696998b-v58bh")

			if !xHasContainer || !yHasContainer {
				t.Error("Expected all petshop-frontend-girls ReplicaSet pods to be expanded with container details")
			}
		}

		if strings.Contains(output, "container ") {
			if !strings.Contains(output, "image ") {
				t.Error("Expected container to show image")
			}
		}

		if strings.Contains(output, "endpointslice ") {
			if !strings.Contains(output, "addressType=") {
				t.Error("Expected endpointslice to show 'addressType=' metadata")
			}
			if !strings.Contains(output, "portName=") {
				t.Error("Expected endpointslice to show 'portName=' metadata")
			}
			if !strings.Contains(output, "port=") {
				t.Error("Expected endpointslice to show 'port=' metadata")
			}
			if !strings.Contains(output, "protocol=") {
				t.Error("Expected endpointslice to show 'protocol=' metadata")
			}
		}

		if !strings.Contains(output, "allow-external-egress") {
			t.Error("Expected standalone CiliumNetworkPolicy 'allow-external-egress' to appear in output")
		}
		if !strings.Contains(output, "kube-root-ca.crt") {
			t.Error("Expected standalone ConfigMap 'kube-root-ca.crt' to appear in output")
		}
		if !strings.Contains(output, "serviceaccount default") {
			t.Error("Expected standalone ServiceAccount 'default' to appear in output")
		}
	})
}
