package mock

import (
	"fmt"
	"time"

	kube "github.com/karloie/kompass/pkg/kube"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

type appSpec struct {
	name         string
	podUID       string
	rsHash       string
	podIP        string
	image        string
	envVars      []corev1.EnvVar
	volumes      []corev1.Volume
	volumeMounts []corev1.VolumeMount
	ports        []corev1.ContainerPort
	hostNetwork  bool
	dnsPolicy    corev1.DNSPolicy
	hostname     string
	subdomain    string
}

func addAppWithSpecs(model *kube.InMemoryModel, spec appSpec) {
	namespace := "petshop"
	replicaSetName := spec.name + "-" + spec.rsHash
	podName := replicaSetName + "-tr5ft"

	deploymentUID := types.UID("dep-" + spec.podUID[:8])
	replicaSetUID := types.UID("rs-" + spec.podUID[:8])
	serviceUID := types.UID("svc-" + spec.podUID[:8])
	serviceAccountUID := types.UID("sa-" + spec.podUID[:8])
	secretUID := types.UID("sec-" + spec.podUID[:8])
	endpointSliceUID := types.UID("eps-" + spec.podUID[:8])

	labels := appLabels(spec.name)
	labelsWithHash := appLabelsWithHash(spec.name, spec.rsHash)

	model.ServiceAccounts = append(model.ServiceAccounts, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.name,
			Namespace: namespace,
			UID:       serviceAccountUID,
			Labels:    labels,
		},
		AutomountServiceAccountToken: ptr.To(false),
	})

	secretData := make(map[string][]byte)
	for _, env := range spec.envVars {
		if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
			secretData[env.ValueFrom.SecretKeyRef.Key] = []byte("ZGVtby1zZWNyZXQ=")
		}
	}

	if len(secretData) > 0 {
		model.Secrets = append(model.Secrets, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      spec.name + "-secrets",
				Namespace: namespace,
				UID:       secretUID,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "apps/v1",
						Kind:       "ReplicaSet",
						Name:       replicaSetName,
						UID:        replicaSetUID,
					},
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: secretData,
		})
	}

	model.Deployments = append(model.Deployments, &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.name,
			Namespace: namespace,
			UID:       deploymentUID,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/instance": spec.name,
					"app.kubernetes.io/name":     spec.name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: buildAppPodSpec(spec, ""),
			},
		},
		Status: appsv1.DeploymentStatus{
			Replicas:            1,
			UpdatedReplicas:     1,
			ReadyReplicas:       1,
			AvailableReplicas:   1,
			UnavailableReplicas: 0,
			Conditions: []appsv1.DeploymentCondition{
				{
					Type:   appsv1.DeploymentAvailable,
					Status: corev1.ConditionTrue,
					Reason: "MinimumReplicasAvailable",
				},
				{
					Type:   appsv1.DeploymentProgressing,
					Status: corev1.ConditionTrue,
					Reason: "NewReplicaSetAvailable",
				},
			},
		},
	})

	model.ReplicaSets = append(model.ReplicaSets, &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      replicaSetName,
			Namespace: namespace,
			UID:       replicaSetUID,
			Labels:    labelsWithHash,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "apps/v1",
					Kind:               "Deployment",
					Name:               spec.name,
					UID:                deploymentUID,
					BlockOwnerDeletion: ptr.To(true),
					Controller:         ptr.To(true),
				},
			},
		},
		Spec: appsv1.ReplicaSetSpec{
			Replicas: ptr.To(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/instance": spec.name,
					"app.kubernetes.io/name":     spec.name,
					"pod-template-hash":          spec.rsHash,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labelsWithHash,
				},
				Spec: buildAppPodSpec(spec, ""),
			},
		},
		Status: appsv1.ReplicaSetStatus{
			Replicas:          1,
			ReadyReplicas:     1,
			AvailableReplicas: 1,
		},
	})

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			UID:       types.UID(spec.podUID),
			Labels:    labelsWithHash,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "apps/v1",
					Kind:               "ReplicaSet",
					Name:               replicaSetName,
					UID:                replicaSetUID,
					BlockOwnerDeletion: ptr.To(true),
					Controller:         ptr.To(true),
				},
			},
		},
		Spec: buildAppPodSpec(spec, "psb-01-worker-055ceed2"),
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: spec.podIP,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:    "app",
					Ready:   true,
					Started: ptr.To(true),
					Image:   spec.image,
					ImageID: spec.image + "@sha256:mock-" + spec.rsHash,
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(time.Date(2026, time.March, 12, 10, 0, 0, 0, time.UTC))},
					},
				},
			},
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
	model.Pods = append(model.Pods, pod)
	addPodRuntimeData(model, pod, spec.name)

	model.Services = append(model.Services, &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.name,
			Namespace: namespace,
			UID:       serviceUID,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"app.kubernetes.io/instance": spec.name,
				"app.kubernetes.io/name":     spec.name,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       spec.name + "-http",
					Port:       8080,
					TargetPort: intstr.FromInt32(8080),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	})

	model.Endpoints = append(model.Endpoints, &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.name,
			Namespace: namespace,
			Labels:    labels,
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: []corev1.EndpointAddress{
					{IP: spec.podIP, NodeName: ptr.To("psb-01-worker-055ceed2"), TargetRef: &corev1.ObjectReference{
						Kind:      "Pod",
						Name:      podName,
						Namespace: namespace,
						UID:       types.UID(spec.podUID),
					}},
				},
				Ports: []corev1.EndpointPort{
					{Name: spec.name + "-http", Port: 8080, Protocol: corev1.ProtocolTCP},
				},
			},
		},
	})

	model.EndpointSlices = append(model.EndpointSlices, &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.name + "-abcde",
			Namespace: namespace,
			UID:       endpointSliceUID,
			Labels: map[string]string{
				"kubernetes.io/service-name":             spec.name,
				"endpointslice.kubernetes.io/managed-by": "endpointslice-controller.k8s.io",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "v1",
					Kind:               "Service",
					Name:               spec.name,
					UID:                serviceUID,
					BlockOwnerDeletion: ptr.To(true),
					Controller:         ptr.To(true),
				},
			},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{spec.podIP},
				Conditions: discoveryv1.EndpointConditions{
					Ready:       ptr.To(true),
					Serving:     ptr.To(true),
					Terminating: ptr.To(false),
				},
				NodeName: ptr.To("psb-01-worker-055ceed2"),
				TargetRef: &corev1.ObjectReference{
					Kind:      "Pod",
					Name:      podName,
					Namespace: namespace,
					UID:       types.UID(spec.podUID),
				},
			},
		},
		Ports: []discoveryv1.EndpointPort{
			{
				Name:     ptr.To(spec.name + "-http"),
				Port:     ptr.To(int32(8080)),
				Protocol: ptr.To(corev1.ProtocolTCP),
			},
		},
	})
}

func addPodRuntimeData(model *kube.InMemoryModel, pod *corev1.Pod, appName string) {
	if model == nil || pod == nil {
		return
	}
	if model.PodLogs == nil {
		model.PodLogs = map[string]string{}
	}

	containerName := "app"
	image := "unknown"
	if len(pod.Spec.Containers) > 0 {
		containerName = pod.Spec.Containers[0].Name
		image = pod.Spec.Containers[0].Image
	}
	if appName == "" {
		appName = pod.Labels["app.kubernetes.io/name"]
	}
	if appName == "" {
		appName = pod.Name
	}

	podRef := pod.Namespace + "/" + pod.Name
	model.PodLogs[podRef] = fmt.Sprintf(
		"2026-03-12T10:00:00Z starting %s\n2026-03-12T10:00:02Z loaded configuration for %s\n2026-03-12T10:00:04Z listening on :8080\n",
		containerName,
		appName,
	)

	startedAt := metav1.NewTime(time.Date(2026, time.March, 12, 10, 0, 0, 0, time.UTC))
	model.Events = append(model.Events,
		&corev1.Event{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: pod.Namespace,
				Name:      pod.Name + ".scheduled",
			},
			InvolvedObject: corev1.ObjectReference{
				Kind:      "Pod",
				Name:      pod.Name,
				Namespace: pod.Namespace,
				UID:       pod.UID,
			},
			Type:           "Normal",
			Reason:         "Scheduled",
			Message:        "Successfully assigned " + pod.Namespace + "/" + pod.Name + " to psb-01-worker-055ceed2",
			FirstTimestamp: startedAt,
			LastTimestamp:  startedAt,
			Count:          1,
		},
		&corev1.Event{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: pod.Namespace,
				Name:      pod.Name + ".pulled",
			},
			InvolvedObject: corev1.ObjectReference{
				Kind:      "Pod",
				Name:      pod.Name,
				Namespace: pod.Namespace,
				UID:       pod.UID,
			},
			Type:           "Normal",
			Reason:         "Pulled",
			Message:        fmt.Sprintf("Container image %q already present on machine", image),
			FirstTimestamp: startedAt,
			LastTimestamp:  startedAt,
			Count:          1,
		},
		&corev1.Event{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: pod.Namespace,
				Name:      pod.Name + ".started",
			},
			InvolvedObject: corev1.ObjectReference{
				Kind:      "Pod",
				Name:      pod.Name,
				Namespace: pod.Namespace,
				UID:       pod.UID,
			},
			Type:           "Normal",
			Reason:         "Started",
			Message:        "Started container " + containerName,
			FirstTimestamp: startedAt,
			LastTimestamp:  startedAt,
			Count:          1,
		},
	)
}

func appLabels(name string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/instance":   name,
		"app.kubernetes.io/name":       name,
		"app.kubernetes.io/managed-by": "Helm",
		"helm.sh/chart":                "petshop-app-helm-chart-0.18.1",
	}
}

func appLabelsWithHash(name, rsHash string) map[string]string {
	labels := appLabels(name)
	labels["pod-template-hash"] = rsHash
	return labels
}

func buildAppContainer(spec appSpec) corev1.Container {
	return corev1.Container{
		Name:            "app",
		Image:           spec.image,
		ImagePullPolicy: corev1.PullAlways,
		Env:             spec.envVars,
		VolumeMounts:    spec.volumeMounts,
		Ports:           getContainerPorts(spec.ports),
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("512Mi"),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
		},
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: ptr.To(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
			},
			ReadOnlyRootFilesystem: ptr.To(false),
			RunAsNonRoot:           ptr.To(true),
			RunAsUser:              ptr.To(int64(1000)),
			SeccompProfile: &corev1.SeccompProfile{
				Type: corev1.SeccompProfileTypeRuntimeDefault,
			},
		},
	}
}

func buildAppPodSpec(spec appSpec, nodeName string) corev1.PodSpec {
	podSpec := corev1.PodSpec{
		ServiceAccountName: spec.name,
		ImagePullSecrets: []corev1.LocalObjectReference{
			{Name: "docker-registry-credentials"},
		},
		Containers:  []corev1.Container{buildAppContainer(spec)},
		Volumes:     spec.volumes,
		HostNetwork: spec.hostNetwork,
		DNSPolicy:   getDNSPolicy(spec.dnsPolicy, spec.hostNetwork),
		Hostname:    spec.hostname,
		Subdomain:   spec.subdomain,
	}
	if nodeName != "" {
		podSpec.NodeName = nodeName
	}
	return podSpec
}

func getContainerPorts(ports []corev1.ContainerPort) []corev1.ContainerPort {
	if len(ports) > 0 {
		return ports
	}

	return []corev1.ContainerPort{
		{Name: "http", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
	}
}

func getDNSPolicy(policy corev1.DNSPolicy, hostNetwork bool) corev1.DNSPolicy {
	if policy != "" {
		return policy
	}

	if hostNetwork {
		return corev1.DNSClusterFirstWithHostNet
	}
	return corev1.DNSClusterFirst
}
