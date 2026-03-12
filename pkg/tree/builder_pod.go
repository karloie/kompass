package tree

import (
	"fmt"

	"github.com/karloie/kompass/pkg/graph"
	kube "github.com/karloie/kompass/pkg/kube"
)

func expandResourceFromVolume(parentKey, namespace string, volume map[string]any, graphChildren map[string][]string, state *treeBuildState, nodeMap map[string]kube.Resource) *kube.Tree {
	volumeSources := []struct {
		field   string
		nameKey string
		resType string
	}{
		{"persistentVolumeClaim", "claimName", "persistentvolumeclaim"},
		{"secret", "secretName", "secret"},
		{"configMap", "name", "configmap"},
	}

	for _, vs := range volumeSources {
		if source, ok := graph.M(volume).MapOk(vs.field); ok {
			if name, ok := source.StringOk(vs.nameKey); ok {
				resourceKey := BuildResourceKeyRef(vs.resType, namespace, name)
				if _, exists := nodeMap[resourceKey]; exists && state.CanTraverse(resourceKey) {
					return buildTreeNode(resourceKey, graphChildren, state, nodeMap)
				}
			}
		}
	}
	return nil
}

func expandVolumesAsResources(parentKey, namespace string, volumes []any, graphChildren map[string][]string, state *treeBuildState, nodeMap map[string]kube.Resource) []*kube.Tree {
	nodes := make([]*kube.Tree, 0)
	for _, vol := range volumes {
		if volMap, ok := vol.(map[string]any); ok {
			if node := expandResourceFromVolume(parentKey, namespace, volMap, graphChildren, state, nodeMap); node != nil {
				nodes = append(nodes, node)
			}
		}
	}
	return nodes
}

func buildPodTemplateChildren(templateKey string, namespace string, templateSpec map[string]any, graphChildren map[string][]string, state *treeBuildState, nodeMap map[string]kube.Resource) []*kube.Tree {
	if templateSpec == nil {
		return nil
	}

	builder := NewChildrenBuilder()
	containers, _ := templateSpec["containers"].([]any)
	volumes, _ := templateSpec["volumes"].([]any)

	if securityContext, ok := templateSpec["securityContext"].(map[string]any); ok {
		builder.Add(buildPodSecurityContextNode(templateKey, securityContext))
	}

	if len(containers) == 1 {
		if containerMap, ok := containers[0].(map[string]any); ok {
			containerChildren := buildContainerChildren(templateKey, namespace, 0, containerMap, nil, volumes, graphChildren, state, nodeMap)
			builder.Extend(containerChildren)
		}
	} else {
		for idx, c := range containers {
			if containerMap, ok := c.(map[string]any); ok {
				builder.Add(buildContainerNode(templateKey, namespace, idx, containerMap, nil, volumes, graphChildren, state, nodeMap))
			}
		}
	}

	builder.Extend(expandVolumesAsResources(templateKey, namespace, volumes, graphChildren, state, nodeMap))

	return builder.Build()
}

func buildPodWithSimplifiedContainers(podKey string, pod kube.Resource) *kube.Tree {
	metadata := graph.M(pod.AsMap()).Map("metadata").Raw()
	status := graph.M(pod.AsMap()).Map("status").Raw()
	spec := graph.M(pod.AsMap()).Map("spec").Raw()

	nodeMetadata := map[string]any{}

	if metadata != nil {
		if name := graph.M(metadata).String("name"); name != "" {
			nodeMetadata["name"] = name
		}
	}

	if status != nil {
		if phase := graph.M(status).String("phase"); phase != "" {
			nodeMetadata["phase"] = phase
		}
		if podIP := graph.M(status).String("podIP"); podIP != "" {
			nodeMetadata["podIP"] = podIP
		}
		if hostIP := graph.M(status).String("hostIP"); hostIP != "" {
			nodeMetadata["hostIP"] = hostIP
		}
	}

	if spec != nil {
		if nodeName := graph.M(spec).String("nodeName"); nodeName != "" {
			nodeMetadata["nodeName"] = nodeName
		}
	}

	podNode := &kube.Tree{
		Key:      podKey,
		Type:     "pod",
		Meta:     nodeMetadata,
		Children: []*kube.Tree{},
	}

	containerStatuses := make(map[string]map[string]any)
	if status != nil {
		if statuses, ok := status["containerStatuses"].([]any); ok {
			for _, s := range statuses {
				if statusMap, ok := s.(map[string]any); ok {
					if name, ok := statusMap["name"].(string); ok {
						containerStatuses[name] = statusMap
					}
				}
			}
		}
	}

	if spec != nil {
		if containers, ok := spec["containers"].([]any); ok {
			for idx, c := range containers {
				if containerMap, ok := c.(map[string]any); ok {
					containerName, _ := containerMap["name"].(string)
					containerStatus := containerStatuses[containerName]

					containerKey := fmt.Sprintf("%s/container/%d", podKey, idx)
					containerNode := NewTree(containerKey, "container", map[string]any{"name": containerName})

					if image, ok := containerMap["image"].(string); ok {
						imageNode := NewTree(containerKey+"/image", "image", map[string]any{"name": image})
						containerNode.Children = append(containerNode.Children, imageNode)
					}

					if containerStatus != nil {
						hasLivenessProbe := containerMap["livenessProbe"] != nil
						hasReadinessProbe := containerMap["readinessProbe"] != nil
						hasStartupProbe := containerMap["startupProbe"] != nil

						if hasLivenessProbe {
							probeStatus := "passing"
							if state, ok := containerStatus["state"].(map[string]any); ok {
								if _, running := state["running"]; !running {
									probeStatus = "unknown"
								}
							}
							livenessNode := NewTree(containerKey+"/livenessprobe", "livenessprobe", map[string]any{"status": probeStatus})
							containerNode.Children = append(containerNode.Children, livenessNode)
						}

						if hasReadinessProbe {
							probeStatus := "not-ready"
							if ready, ok := containerStatus["ready"].(bool); ok && ready {
								probeStatus = "ready"
							}
							readinessNode := NewTree(containerKey+"/readinessprobe", "readinessprobe", map[string]any{"status": probeStatus})
							containerNode.Children = append(containerNode.Children, readinessNode)
						}

						if hasStartupProbe {
							probeStatus := "not-started"
							if started, ok := containerStatus["started"].(bool); ok && started {
								probeStatus = "started"
							}
							startupNode := NewTree(containerKey+"/startupprobe", "startupprobe", map[string]any{"status": probeStatus})
							containerNode.Children = append(containerNode.Children, startupNode)
						}
					}

					podNode.Children = append(podNode.Children, containerNode)
				}
			}
		}
	}

	return podNode
}

func buildSimplifiedPodNode(podKey string, pod kube.Resource) *kube.Tree {
	metadata := graph.M(pod.AsMap()).Map("metadata").Raw()
	status := graph.M(pod.AsMap()).Map("status").Raw()
	spec := graph.M(pod.AsMap()).Map("spec").Raw()

	nodeMetadata := map[string]any{}

	if metadata != nil {
		if name := graph.M(metadata).String("name"); name != "" {
			nodeMetadata["name"] = name
		}
	}

	if status != nil {
		if phase := graph.M(status).String("phase"); phase != "" {
			nodeMetadata["phase"] = phase
		}
		if podIP := graph.M(status).String("podIP"); podIP != "" {
			nodeMetadata["podIP"] = podIP
		}
		if hostIP := graph.M(status).String("hostIP"); hostIP != "" {
			nodeMetadata["hostIP"] = hostIP
		}
	}

	if spec != nil {
		if nodeName := graph.M(spec).String("nodeName"); nodeName != "" {
			nodeMetadata["nodeName"] = nodeName
		}
	}

	return &kube.Tree{
		Key:      podKey,
		Type:     "pod",
		Meta:     nodeMetadata,
		Children: []*kube.Tree{},
	}
}

func buildPodChildren(podKey string, pod kube.Resource, graphChildren map[string][]string, state *treeBuildState, nodeMap map[string]kube.Resource) []*kube.Tree {
	spec := graph.M(pod.AsMap()).Map("spec").Raw()
	metadata := graph.M(pod.AsMap()).Map("metadata").Raw()
	status := graph.M(pod.AsMap()).Map("status").Raw()
	namespace := graph.M(metadata).String("namespace")

	if spec == nil {
		return nil
	}

	builder := NewChildrenBuilder()
	specKey := podKey + "/spec"
	specNode := NewTree(specKey, "spec", map[string]any{})
	specBuilder := NewChildrenBuilder()
	containers, _ := spec["containers"].([]any)
	volumes, _ := spec["volumes"].([]any)

	if securityContext, ok := spec["securityContext"].(map[string]any); ok {
		builder.Add(buildPodSecurityContextNode(podKey, securityContext))
	}

	if len(containers) == 1 {
		if containerMap, ok := containers[0].(map[string]any); ok {
			specBuilder.Extend(buildContainerChildren(specKey, namespace, 0, containerMap, nil, volumes, graphChildren, state, nodeMap))
		}
	} else {
		for idx, c := range containers {
			if containerMap, ok := c.(map[string]any); ok {
				specBuilder.Add(buildContainerNode(specKey, namespace, idx, containerMap, nil, volumes, graphChildren, state, nodeMap))
			}
		}
	}

	specBuilder.Extend(expandVolumesAsResources(specKey, namespace, volumes, graphChildren, state, nodeMap))
	specNode.Children = specBuilder.Build()
	builder.Extend(buildRuntimeContainerChildren(podKey, spec, status))
	builder.Add(specNode)

	builder.Extend(appendGraphChildren(podKey, graphChildren, state, nodeMap))

	return builder.Build()
}

func buildRuntimeContainerChildren(podKey string, spec map[string]any, status map[string]any) []*kube.Tree {
	if spec == nil {
		return nil
	}

	containerStatuses := make(map[string]map[string]any)
	if status != nil {
		if statuses, ok := status["containerStatuses"].([]any); ok {
			for _, s := range statuses {
				if statusMap, ok := s.(map[string]any); ok {
					if name, ok := statusMap["name"].(string); ok {
						containerStatuses[name] = statusMap
					}
				}
			}
		}
	}

	runtimeChildren := make([]*kube.Tree, 0)
	if containers, ok := spec["containers"].([]any); ok {
		for idx, c := range containers {
			containerMap, ok := c.(map[string]any)
			if !ok {
				continue
			}

			containerName, _ := containerMap["name"].(string)
			containerStatus := containerStatuses[containerName]
			containerKey := fmt.Sprintf("%s/container/%d", podKey, idx)

			metadata := map[string]any{"name": containerName}
			if containerStatus != nil {
				if restartCount, ok := containerStatus["restartCount"].(float64); ok && restartCount > 0 {
					metadata["restarts"] = fmt.Sprintf("%.0f", restartCount)
				}
				if state, ok := containerStatus["state"].(map[string]any); ok {
					if waiting, ok := state["waiting"].(map[string]any); ok {
						metadata["state"] = "Waiting"
						if reason, ok := waiting["reason"].(string); ok && reason != "" {
							metadata["reason"] = reason
						}
					} else if running, ok := state["running"].(map[string]any); ok {
						if startedAt, ok := running["startedAt"].(string); ok && startedAt != "" {
							metadata["state"] = "Running"
						}
					} else if terminated, ok := state["terminated"].(map[string]any); ok {
						metadata["state"] = "Terminated"
						if exitCode, ok := terminated["exitCode"].(float64); ok {
							metadata["exitCode"] = fmt.Sprintf("%.0f", exitCode)
						}
						if reason, ok := terminated["reason"].(string); ok && reason != "" {
							metadata["reason"] = reason
						}
					}
				}
			}

			containerNode := NewTree(containerKey, "container", metadata)
			if image, ok := containerMap["image"].(string); ok {
				containerNode.Children = append(containerNode.Children, NewTree(containerKey+"/image", "image", map[string]any{"name": image}))
			}
			runtimeChildren = append(runtimeChildren, containerNode)
		}
	}

	return runtimeChildren
}

func buildContainerChildren(parentKey string, namespace string, idx int, containerSpec map[string]any, containerStatus map[string]any, volumes []any, graphChildren map[string][]string, state *treeBuildState, nodeMap map[string]kube.Resource) []*kube.Tree {
	containerKey := fmt.Sprintf("%s/container/%d", parentKey, idx)
	var children []*kube.Tree

	if image, ok := containerSpec["image"].(string); ok {
		imageMetadata := map[string]any{"name": image}
		if pullPolicy, ok := containerSpec["imagePullPolicy"].(string); ok && pullPolicy != "" {
			imageMetadata["pullPolicy"] = pullPolicy
		}
		imageNode := NewTree(containerKey+"/image", "image", imageMetadata)
		children = append(children, imageNode)
	}

	if ports, ok := containerSpec["ports"].([]any); ok && len(ports) > 0 {
		portsNode := buildPortsNode(containerKey, ports)
		if portsNode != nil {
			children = append(children, portsNode)
		}
	}

	if envVars, ok := containerSpec["env"].([]any); ok && len(envVars) > 0 {
		envarsNode := buildEnvarsNode(containerKey, namespace, envVars, containerSpec, nodeMap)
		if envarsNode != nil {
			children = append(children, envarsNode)
		}
	}

	if volumeMounts, ok := containerSpec["volumeMounts"].([]any); ok && len(volumeMounts) > 0 {
		mountsNode := buildMountsNode(containerKey, namespace, volumeMounts, volumes, graphChildren, state, nodeMap)
		if mountsNode != nil {
			children = append(children, mountsNode)
		}
	}

	if resources, ok := containerSpec["resources"].(map[string]any); ok && len(resources) > 0 {
		resourcesNode := buildResourcesNode(containerKey, resources)
		if resourcesNode != nil {
			children = append(children, resourcesNode)
		}
	}

	if livenessProbe, ok := containerSpec["livenessProbe"].(map[string]any); ok {
		probeNode := buildProbeNode(containerKey, "livenessprobe", livenessProbe, containerStatus)
		if probeNode != nil {
			children = append(children, probeNode)
		}
	}

	if readinessProbe, ok := containerSpec["readinessProbe"].(map[string]any); ok {
		probeNode := buildProbeNode(containerKey, "readinessprobe", readinessProbe, containerStatus)
		if probeNode != nil {
			children = append(children, probeNode)
		}
	}

	if startupProbe, ok := containerSpec["startupProbe"].(map[string]any); ok {
		probeNode := buildProbeNode(containerKey, "startupprobe", startupProbe, containerStatus)
		if probeNode != nil {
			children = append(children, probeNode)
		}
	}

	if securityContext, ok := containerSpec["securityContext"].(map[string]any); ok {
		securityContextNode := buildSecurityContextNode(containerKey, securityContext)
		if securityContextNode != nil {
			children = append(children, securityContextNode)
		}
	}

	return children
}

func buildContainerNode(podKey string, namespace string, idx int, containerSpec map[string]any, containerStatus map[string]any, volumes []any, graphChildren map[string][]string, state *treeBuildState, nodeMap map[string]kube.Resource) *kube.Tree {
	containerName, _ := containerSpec["name"].(string)
	containerKey := fmt.Sprintf("%s/container/%d", podKey, idx)

	metadata := map[string]any{
		"name": containerName,
	}

	if resources, ok := containerSpec["resources"].(map[string]any); ok {
		var cpuReq, cpuLim, memReq, memLim string
		if requests, ok := resources["requests"].(map[string]any); ok {
			if cpu, ok := requests["cpu"].(string); ok && cpu != "" {
				cpuReq = cpu
			}
			if mem, ok := requests["memory"].(string); ok && mem != "" {
				memReq = mem
			}
		}
		if limits, ok := resources["limits"].(map[string]any); ok {
			if cpu, ok := limits["cpu"].(string); ok && cpu != "" {
				cpuLim = cpu
			}
			if mem, ok := limits["memory"].(string); ok && mem != "" {
				memLim = mem
			}
		}
		if cpuReq != "" || cpuLim != "" {
			if cpuReq != "" && cpuLim != "" {
				metadata["cpu"] = fmt.Sprintf("%s/%s", cpuReq, cpuLim)
			} else if cpuReq != "" {
				metadata["cpu"] = cpuReq
			} else {
				metadata["cpu"] = cpuLim
			}
		}
		if memReq != "" || memLim != "" {
			if memReq != "" && memLim != "" {
				metadata["memory"] = fmt.Sprintf("%s/%s", memReq, memLim)
			} else if memReq != "" {
				metadata["memory"] = memReq
			} else {
				metadata["memory"] = memLim
			}
		}
	}

	if containerStatus != nil {
		if restartCount, ok := containerStatus["restartCount"].(float64); ok && restartCount > 0 {
			metadata["restarts"] = fmt.Sprintf("%.0f", restartCount)
		}
		if state, ok := containerStatus["state"].(map[string]any); ok {
			if waiting, ok := state["waiting"].(map[string]any); ok {
				metadata["state"] = "Waiting"
				if reason, ok := waiting["reason"].(string); ok && reason != "" {
					metadata["reason"] = reason
				}
			} else if running, ok := state["running"].(map[string]any); ok {
				if startedAt, ok := running["startedAt"].(string); ok && startedAt != "" {
					metadata["state"] = "Running"
				}
			} else if terminated, ok := state["terminated"].(map[string]any); ok {
				metadata["state"] = "Terminated"
				if exitCode, ok := terminated["exitCode"].(float64); ok {
					metadata["exitCode"] = fmt.Sprintf("%.0f", exitCode)
				}
				if reason, ok := terminated["reason"].(string); ok && reason != "" {
					metadata["reason"] = reason
				}
			}
		}
	}

	containerNode := NewTree(containerKey, "container", metadata)

	containerNode.Children = buildContainerChildren(podKey, namespace, idx, containerSpec, containerStatus, volumes, graphChildren, state, nodeMap)

	return containerNode
}

func buildSecurityContextNode(containerKey string, securityContext map[string]any) *kube.Tree {
	securityContextKey := containerKey + "/securityContext"
	metadata := map[string]any{}

	if runAsUser, ok := graph.M(securityContext).IntOk("runAsUser"); ok {
		metadata["runAsUser"] = runAsUser
	}
	if runAsGroup, ok := graph.M(securityContext).IntOk("runAsGroup"); ok {
		metadata["runAsGroup"] = runAsGroup
	}
	if runAsNonRoot, ok := securityContext["runAsNonRoot"].(bool); ok {
		metadata["runAsNonRoot"] = runAsNonRoot
	}
	if privileged, ok := securityContext["privileged"].(bool); ok {
		metadata["privileged"] = privileged
	}
	if allowPrivilegeEscalation, ok := securityContext["allowPrivilegeEscalation"].(bool); ok {
		metadata["allowPrivilegeEscalation"] = allowPrivilegeEscalation
	}
	if readOnlyRootFilesystem, ok := securityContext["readOnlyRootFilesystem"].(bool); ok {
		metadata["readOnlyRootFilesystem"] = readOnlyRootFilesystem
	}
	if procMount, ok := securityContext["procMount"].(string); ok && procMount != "Default" {
		metadata["procMount"] = procMount
	}

	if capabilities, ok := securityContext["capabilities"].(map[string]any); ok {
		if add, ok := capabilities["add"].([]any); ok && len(add) > 0 {
			addStrs := []string{}
			for _, cap := range add {
				if capStr, ok := cap.(string); ok {
					addStrs = append(addStrs, capStr)
				}
			}
			if len(addStrs) > 0 {
				metadata["capabilitiesAdd"] = addStrs
			}
		}
		if drop, ok := capabilities["drop"].([]any); ok && len(drop) > 0 {
			dropStrs := []string{}
			for _, cap := range drop {
				if capStr, ok := cap.(string); ok {
					dropStrs = append(dropStrs, capStr)
				}
			}
			if len(dropStrs) > 0 {
				metadata["capabilitiesDrop"] = dropStrs
			}
		}
	}

	if seccompProfile, ok := securityContext["seccompProfile"].(map[string]any); ok {
		if profileType, ok := seccompProfile["type"].(string); ok {
			metadata["seccompProfile"] = profileType
			if profileType == "Localhost" {
				if localhostProfile, ok := seccompProfile["localhostProfile"].(string); ok {
					metadata["seccompLocalhostProfile"] = localhostProfile
				}
			}
		}
	}

	if seLinuxOptions, ok := securityContext["seLinuxOptions"].(map[string]any); ok {
		if level, ok := seLinuxOptions["level"].(string); ok {
			metadata["seLinuxLevel"] = level
		}
		if role, ok := seLinuxOptions["role"].(string); ok {
			metadata["seLinuxRole"] = role
		}
		if seLinuxType, ok := seLinuxOptions["type"].(string); ok {
			metadata["seLinuxType"] = seLinuxType
		}
		if user, ok := seLinuxOptions["user"].(string); ok {
			metadata["seLinuxUser"] = user
		}
	}

	if windowsOptions, ok := securityContext["windowsOptions"].(map[string]any); ok {
		if gmsaCredentialSpec, ok := windowsOptions["gmsaCredentialSpec"].(string); ok {
			metadata["windowsGmsaCredentialSpec"] = gmsaCredentialSpec
		}
		if gmsaCredentialSpecName, ok := windowsOptions["gmsaCredentialSpecName"].(string); ok {
			metadata["windowsGmsaCredentialSpecName"] = gmsaCredentialSpecName
		}
		if runAsUserName, ok := windowsOptions["runAsUserName"].(string); ok {
			metadata["windowsRunAsUserName"] = runAsUserName
		}
		if hostProcess, ok := windowsOptions["hostProcess"].(bool); ok {
			metadata["windowsHostProcess"] = hostProcess
		}
	}

	if len(metadata) == 0 {
		return nil
	}

	return NewTree(securityContextKey, "securitycontext", metadata)
}

func buildPodSecurityContextNode(podKey string, securityContext map[string]any) *kube.Tree {
	securityContextKey := podKey + "/securityContext"
	metadata := map[string]any{}

	if runAsUser, ok := graph.M(securityContext).IntOk("runAsUser"); ok {
		metadata["runAsUser"] = runAsUser
	}
	if runAsGroup, ok := graph.M(securityContext).IntOk("runAsGroup"); ok {
		metadata["runAsGroup"] = runAsGroup
	}
	if runAsNonRoot, ok := securityContext["runAsNonRoot"].(bool); ok {
		metadata["runAsNonRoot"] = runAsNonRoot
	}
	if fsGroup, ok := graph.M(securityContext).IntOk("fsGroup"); ok {
		metadata["fsGroup"] = fsGroup
	}
	if fsGroupChangePolicy, ok := securityContext["fsGroupChangePolicy"].(string); ok {
		metadata["fsGroupChangePolicy"] = fsGroupChangePolicy
	}
	if sysctls, ok := securityContext["sysctls"].([]any); ok && len(sysctls) > 0 {
		sysctlStrs := []string{}
		for _, sysctl := range sysctls {
			if sysctlMap, ok := sysctl.(map[string]any); ok {
				if name, ok := sysctlMap["name"].(string); ok {
					if value, ok := sysctlMap["value"].(string); ok {
						sysctlStrs = append(sysctlStrs, name+"="+value)
					}
				}
			}
		}
		if len(sysctlStrs) > 0 {
			metadata["sysctls"] = sysctlStrs
		}
	}

	if supplementalGroups, ok := securityContext["supplementalGroups"].([]any); ok && len(supplementalGroups) > 0 {
		groups := []int{}
		for _, g := range supplementalGroups {
			if gid, ok := graph.M(map[string]any{"v": g}).IntOk("v"); ok {
				groups = append(groups, gid)
			}
		}
		if len(groups) > 0 {
			metadata["supplementalGroups"] = groups
		}
	}

	if seccompProfile, ok := securityContext["seccompProfile"].(map[string]any); ok {
		if profileType, ok := seccompProfile["type"].(string); ok {
			metadata["seccompProfile"] = profileType
			if profileType == "Localhost" {
				if localhostProfile, ok := seccompProfile["localhostProfile"].(string); ok {
					metadata["seccompLocalhostProfile"] = localhostProfile
				}
			}
		}
	}

	if seLinuxOptions, ok := securityContext["seLinuxOptions"].(map[string]any); ok {
		if level, ok := seLinuxOptions["level"].(string); ok {
			metadata["seLinuxLevel"] = level
		}
		if role, ok := seLinuxOptions["role"].(string); ok {
			metadata["seLinuxRole"] = role
		}
		if seLinuxType, ok := seLinuxOptions["type"].(string); ok {
			metadata["seLinuxType"] = seLinuxType
		}
		if user, ok := seLinuxOptions["user"].(string); ok {
			metadata["seLinuxUser"] = user
		}
	}

	if windowsOptions, ok := securityContext["windowsOptions"].(map[string]any); ok {
		if gmsaCredentialSpec, ok := windowsOptions["gmsaCredentialSpec"].(string); ok {
			metadata["windowsGmsaCredentialSpec"] = gmsaCredentialSpec
		}
		if gmsaCredentialSpecName, ok := windowsOptions["gmsaCredentialSpecName"].(string); ok {
			metadata["windowsGmsaCredentialSpecName"] = gmsaCredentialSpecName
		}
		if runAsUserName, ok := windowsOptions["runAsUserName"].(string); ok {
			metadata["windowsRunAsUserName"] = runAsUserName
		}
		if hostProcess, ok := windowsOptions["hostProcess"].(bool); ok {
			metadata["windowsHostProcess"] = hostProcess
		}
	}

	if len(metadata) == 0 {
		return nil
	}

	return NewTree(securityContextKey, "podsecuritycontext", metadata)
}

func buildPortsNode(containerKey string, ports []any) *kube.Tree {
	portsKey := containerKey + "/ports"

	portsNode := NewTree(portsKey, "ports", nil)

	for idx, p := range ports {
		if portMap, ok := p.(map[string]any); ok {
			portKey := fmt.Sprintf("%s/port/%d", portsKey, idx)

			metadata := map[string]any{}

			if containerPort, ok := graph.M(portMap).IntOk("containerPort"); ok {
				metadata["containerPort"] = containerPort
			}
			if protocol, ok := graph.M(portMap).StringOk("protocol"); ok {
				metadata["protocol"] = protocol
			} else {
				metadata["protocol"] = "TCP"
			}
			if name, ok := graph.M(portMap).StringOk("name"); ok {
				metadata["name"] = name
			}
			if hostPort, ok := graph.M(portMap).IntOk("hostPort"); ok {
				metadata["hostPort"] = hostPort
			}

			portNode := NewTree(portKey, "port", metadata)
			portsNode.Children = append(portsNode.Children, portNode)
		}
	}

	return portsNode
}

func buildResourcesNode(containerKey string, resources map[string]any) *kube.Tree {
	resourcesKey := containerKey + "/resources"

	metadata := map[string]any{}

	if limits, ok := resources["limits"].(map[string]any); ok && len(limits) > 0 {
		limitsMap := make(map[string]any)
		for k, v := range limits {
			if str, ok := v.(string); ok {
				limitsMap[k] = str
			}
		}
		if len(limitsMap) > 0 {
			metadata["limits"] = limitsMap
		}
	}

	if requests, ok := resources["requests"].(map[string]any); ok && len(requests) > 0 {
		requestsMap := make(map[string]any)
		for k, v := range requests {
			if str, ok := v.(string); ok {
				requestsMap[k] = str
			}
		}
		if len(requestsMap) > 0 {
			metadata["requests"] = requestsMap
		}
	}

	if len(metadata) == 0 {
		return nil
	}

	return NewTree(resourcesKey, "resources", metadata)
}

func buildProbeNode(containerKey string, probeType string, probe map[string]any, containerStatus map[string]any) *kube.Tree {
	probeKey := containerKey + "/" + probeType

	metadata := map[string]any{}

	if containerStatus != nil {
		if probeType == "readinessprobe" {
			if ready, ok := containerStatus["ready"].(bool); ok {
				if ready {
					metadata["status"] = "ready"
				} else {
					metadata["status"] = "not-ready"
				}
			}
		} else if probeType == "startupprobe" {
			if started, ok := containerStatus["started"].(bool); ok {
				if started {
					metadata["status"] = "started"
				} else {
					metadata["status"] = "not-started"
				}
			}
		} else if probeType == "livenessprobe" {
			if state, ok := containerStatus["state"].(map[string]any); ok {
				if _, running := state["running"]; running {
					metadata["status"] = "passing"
				} else if terminated, ok := state["terminated"].(map[string]any); ok {
					if reason, ok := terminated["reason"].(string); ok && reason != "" {
						metadata["status"] = "failed"
					}
				}
			}
		}
	}

	if httpGet, ok := graph.M(probe).MapOk("httpGet"); ok {
		metadata["type"] = "httpGet"
		if path, ok := httpGet.StringOk("path"); ok {
			metadata["path"] = path
		}
		if port, ok := httpGet.Raw()["port"]; ok {
			metadata["port"] = port
		}
		if scheme, ok := httpGet.StringOk("scheme"); ok && scheme != "HTTP" {
			metadata["scheme"] = scheme
		}
		if host, ok := httpGet.StringOk("host"); ok && host != "" {
			metadata["host"] = host
		}
		if httpHeaders, ok := httpGet.Raw()["httpHeaders"].([]any); ok && len(httpHeaders) > 0 {
			headers := []string{}
			for _, h := range httpHeaders {
				if hMap, ok := h.(map[string]any); ok {
					if name, ok := hMap["name"].(string); ok {
						if value, ok := hMap["value"].(string); ok {
							headers = append(headers, name+": "+value)
						}
					}
				}
			}
			if len(headers) > 0 {
				metadata["httpHeaders"] = headers
			}
		}
	} else if exec, ok := graph.M(probe).MapOk("exec"); ok {
		metadata["type"] = "exec"
		if command, ok := exec.Raw()["command"].([]any); ok && len(command) > 0 {
			cmdStrs := []string{}
			for _, c := range command {
				if str, ok := c.(string); ok {
					cmdStrs = append(cmdStrs, str)
				}
			}
			if len(cmdStrs) > 0 {
				metadata["command"] = cmdStrs
			}
		}
	} else if tcpSocket, ok := graph.M(probe).MapOk("tcpSocket"); ok {
		metadata["type"] = "tcpSocket"
		if port, ok := tcpSocket.Raw()["port"]; ok {
			metadata["port"] = port
		}
	} else if grpc, ok := graph.M(probe).MapOk("grpc"); ok {
		metadata["type"] = "grpc"
		if port, ok := grpc.IntOk("port"); ok {
			metadata["port"] = port
		}
		if service, ok := grpc.StringOk("service"); ok {
			metadata["service"] = service
		}
	}

	if initialDelay, ok := graph.M(probe).IntOk("initialDelaySeconds"); ok && initialDelay > 0 {
		metadata["initialDelaySeconds"] = initialDelay
	}
	if period, ok := graph.M(probe).IntOk("periodSeconds"); ok && period != 10 {
		metadata["periodSeconds"] = period
	}
	if timeout, ok := graph.M(probe).IntOk("timeoutSeconds"); ok && timeout != 1 {
		metadata["timeoutSeconds"] = timeout
	}
	if successThreshold, ok := graph.M(probe).IntOk("successThreshold"); ok && successThreshold != 1 {
		metadata["successThreshold"] = successThreshold
	}
	if failureThreshold, ok := graph.M(probe).IntOk("failureThreshold"); ok && failureThreshold != 3 {
		metadata["failureThreshold"] = failureThreshold
	}
	if terminationGracePeriod, ok := graph.M(probe).IntOk("terminationGracePeriodSeconds"); ok {
		metadata["terminationGracePeriodSeconds"] = terminationGracePeriod
	}

	if len(metadata) == 0 {
		return nil
	}

	return NewTree(probeKey, probeType, metadata)
}
