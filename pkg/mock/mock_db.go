package mock

import (
	kube "github.com/karloie/kompass/pkg/kube"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

func addDBApp(model *kube.InMemoryModel) {
	namespace := "petshop"
	deploymentUID := types.UID("c3efee06-439d-45a0-8791-c28afddf18f5")
	replicaSetUID := types.UID("c9d50bdc-9e23-41d0-8fdc-a106039f0e70")
	replicaSetName := "petshop-db-5cb9cd8b74"
	podUID := types.UID("a3a5cea1-2b55-41de-8612-842f1bb9c7e7")
	podName := "petshop-db-5cb9cd8b74-pqhk9"
	podIP := "10.244.9.90"
	serviceUID := types.UID("e29025e1-ad63-4ed8-99d6-5e57c44400f4")
	serviceName := "petshop-db-service"
	pvcUID := types.UID("dbde64d2-ef2b-4cb7-ae0d-a3b07cb7e522")
	pvName := "pvc-dbde64d2-ef2b-4cb7-ae0d-a3b07cb7e522"

	labels := map[string]string{
		"app.kubernetes.io/instance":   "petshop-db",
		"app.kubernetes.io/name":       "petshop-db",
		"app.kubernetes.io/managed-by": "Helm",
		"app.kubernetes.io/version":    "5.26.20-community-ubi9",
		"helm.sh/chart":                "petshop-app-helm-chart-0.18.1",
	}

	labelsWithHash := map[string]string{
		"app.kubernetes.io/instance":   "petshop-db",
		"app.kubernetes.io/name":       "petshop-db",
		"app.kubernetes.io/managed-by": "Helm",
		"app.kubernetes.io/version":    "5.26.20-community-ubi9",
		"helm.sh/chart":                "petshop-app-helm-chart-0.18.1",
		"pod-template-hash":            "5cb9cd8b74",
	}

	model.ServiceAccounts = append(model.ServiceAccounts, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "petshop-db",
			Namespace: namespace,
			UID:       "efd6f752-b072-4d11-83f2-1148f0b3433a",
			Labels:    labels,
		},
		AutomountServiceAccountToken: ptr.To(false),
	})

	model.Secrets = append(model.Secrets, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "petshop-db-secrets",
			Namespace: namespace,
			UID:       "6cfbd2ee-3a98-439c-bd2e-072d36e0f6ab",
			Labels: map[string]string{
				"secrets-store.csi.k8s.io/managed": "true",
			},
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
		Data: map[string][]byte{
			"PETSHOP-DATABASE-PASSWORD": []byte("R1NYZ0xYMVI1WUlhbTNvOG5OaEQ1OWlUY0xia0Za"),
		},
	})

	model.Secrets = append(model.Secrets, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "petshop-db-secret",
			Namespace: namespace,
			UID:       "ce19789a-c10f-45b1-a0ad-856586b6e8cf",
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"password": []byte("dGVzdC1wYXNzd29yZC0xMjM="),
		},
	})

	containerSpec := corev1.Container{
		Name:            "app",
		Image:           "docker-hub/neo4j:5.26.20-community-ubi9",
		ImagePullPolicy: corev1.PullAlways,
		Ports: []corev1.ContainerPort{
			{Name: "http", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
		},
		Env: []corev1.EnvVar{
			{
				Name: "NEO_DB_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "petshop-db-secrets",
						},
						Key: "PETSHOP-DATABASE-PASSWORD",
					},
				},
			},
		},
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("1Gi"),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt32(7687),
				},
			},
			FailureThreshold: 40,
			PeriodSeconds:    5,
			SuccessThreshold: 1,
			TimeoutSeconds:   10,
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt32(7687),
				},
			},
			FailureThreshold: 20,
			PeriodSeconds:    5,
			SuccessThreshold: 1,
			TimeoutSeconds:   10,
		},
		StartupProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt32(7687),
				},
			},
			FailureThreshold: 1000,
			PeriodSeconds:    5,
			SuccessThreshold: 1,
			TimeoutSeconds:   1,
		},
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: ptr.To(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
			},
			ReadOnlyRootFilesystem: ptr.To(false),
			RunAsNonRoot:           ptr.To(true),
			RunAsUser:              ptr.To(int64(7474)),
			SeccompProfile: &corev1.SeccompProfile{
				Type: corev1.SeccompProfileTypeRuntimeDefault,
			},
		},
	}

	podSecurityContext := &corev1.PodSecurityContext{
		FSGroup:    ptr.To(int64(7474)),
		RunAsGroup: ptr.To(int64(7474)),
		RunAsUser:  ptr.To(int64(7474)),
	}

	model.Deployments = append(model.Deployments, &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "petshop-db",
			Namespace: namespace,
			UID:       deploymentUID,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/instance": "petshop-db",
					"app.kubernetes.io/name":     "petshop-db",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "petshop-db",
					SecurityContext:    podSecurityContext,
					Containers:         []corev1.Container{containerSpec},
					Volumes: []corev1.Volume{
						{
							Name: "tmp",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{
									Medium: corev1.StorageMediumMemory,
								},
							},
						},
						{
							Name: "tlosappplatt",
							VolumeSource: corev1.VolumeSource{
								CSI: &corev1.CSIVolumeSource{
									Driver:   "secrets-store.csi.k8s.io",
									ReadOnly: ptr.To(true),
									VolumeAttributes: map[string]string{
										"secretProviderClass": "petshop-db-tlosappplatt",
									},
								},
							},
						},
						{
							Name: "petshop-db-data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "petshop-db-data",
								},
							},
						},
					},
				},
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
					Name:               "petshop-db",
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
					"app.kubernetes.io/instance": "petshop-db",
					"app.kubernetes.io/name":     "petshop-db",
					"pod-template-hash":          "5cb9cd8b74",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labelsWithHash,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "petshop-db",
					SecurityContext:    podSecurityContext,
					Containers:         []corev1.Container{containerSpec},
					Volumes: []corev1.Volume{
						{
							Name: "tmp",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{
									Medium: corev1.StorageMediumMemory,
								},
							},
						},
						{
							Name: "tlosappplatt",
							VolumeSource: corev1.VolumeSource{
								CSI: &corev1.CSIVolumeSource{
									Driver:   "secrets-store.csi.k8s.io",
									ReadOnly: ptr.To(true),
									VolumeAttributes: map[string]string{
										"secretProviderClass": "petshop-db-tlosappplatt",
									},
								},
							},
						},
						{
							Name: "petshop-db-data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "petshop-db-data",
								},
							},
						},
					},
				},
			},
		},
		Status: appsv1.ReplicaSetStatus{
			Replicas:          1,
			ReadyReplicas:     1,
			AvailableReplicas: 1,
		},
	})

	containerSpecWithMounts := containerSpec
	containerSpecWithMounts.VolumeMounts = []corev1.VolumeMount{
		{Name: "tmp", MountPath: "/tmp"},
		{Name: "tlosappplatt", MountPath: "/mnt/secrets", ReadOnly: true},
		{Name: "petshop-db-data", MountPath: "/data"},
	}

	model.Pods = append(model.Pods, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			UID:       podUID,
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
		Spec: corev1.PodSpec{
			NodeName:           "bunny-01-worker-055ceed2",
			ServiceAccountName: "petshop-db",
			SecurityContext:    podSecurityContext,
			Containers:         []corev1.Container{containerSpecWithMounts},
			Volumes: []corev1.Volume{
				{
					Name: "tmp",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{
							Medium: corev1.StorageMediumMemory,
						},
					},
				},
				{
					Name: "tlosappplatt",
					VolumeSource: corev1.VolumeSource{
						CSI: &corev1.CSIVolumeSource{
							Driver:   "secrets-store.csi.k8s.io",
							ReadOnly: ptr.To(true),
							VolumeAttributes: map[string]string{
								"secretProviderClass": "petshop-db-tlosappplatt",
							},
						},
					},
				},
				{
					Name: "petshop-db-data",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "petshop-db-data",
						},
					},
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: podIP,
			PodIPs: []corev1.PodIP{
				{IP: podIP},
			},
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	})

	model.PersistentVolumeClaims = append(model.PersistentVolumeClaims, &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "petshop-db-data",
			Namespace: namespace,
			UID:       pvcUID,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("3Gi"),
				},
			},
			StorageClassName: ptr.To("standard"),
			VolumeMode:       ptr.To(corev1.PersistentVolumeFilesystem),
			VolumeName:       pvName,
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimBound,
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("3Gi"),
			},
		},
	})

	model.PersistentVolumes = append(model.PersistentVolumes, &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: pvName,
			UID:  types.UID("c3c9c3a9-8839-4c7b-b3e8-5a8f9c3c9c3a"),
		},
		Spec: corev1.PersistentVolumeSpec{
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("3Gi"),
			},
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimDelete,
			StorageClassName:              "standard",
			VolumeMode:                    ptr.To(corev1.PersistentVolumeFilesystem),
			ClaimRef: &corev1.ObjectReference{
				Kind:      "PersistentVolumeClaim",
				Namespace: namespace,
				Name:      "petshop-db-data",
				UID:       pvcUID,
			},
		},
		Status: corev1.PersistentVolumeStatus{
			Phase: corev1.VolumeBound,
		},
	})

	model.StorageClasses = append(model.StorageClasses, &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "standard",
			UID:  types.UID("b8f9c3a9-7729-4c6b-a2d7-4a7f8c2c8b2b"),
		},
		Provisioner:          "kubernetes.io/gce-pd",
		ReclaimPolicy:        ptr.To(corev1.PersistentVolumeReclaimDelete),
		VolumeBindingMode:    ptr.To(storagev1.VolumeBindingImmediate),
		AllowVolumeExpansion: ptr.To(true),
	})

	model.VolumeAttachments = append(model.VolumeAttachments, &storagev1.VolumeAttachment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "csi-959030a095e12a5c5224b5fe15796d0ad6ae46b1099c05fc09f46e08a6f47359",
			UID:  types.UID("a7e8b2a8-6618-4b5a-91c6-3a6e7b1b7a1a"),
		},
		Spec: storagev1.VolumeAttachmentSpec{
			Attacher: "pd.csi.storage.gke.io",
			NodeName: "bunny-01-worker-055ceed2",
			Source: storagev1.VolumeAttachmentSource{
				PersistentVolumeName: ptr.To(pvName),
			},
		},
		Status: storagev1.VolumeAttachmentStatus{
			Attached: true,
		},
	})

	model.Services = append(model.Services, &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: namespace,
			UID:       serviceUID,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"app.kubernetes.io/instance": "petshop-db",
				"app.kubernetes.io/name":     "petshop-db",
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "petshop-db-http",
					Port:       7474,
					TargetPort: intstr.FromInt(7474),
					Protocol:   corev1.ProtocolTCP,
				},
				{
					Name:       "petshop-db-bolt",
					Port:       7687,
					TargetPort: intstr.FromInt(7687),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	})

	model.Endpoints = append(model.Endpoints, &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: namespace,
			UID:       "6459a539-dd6e-4a01-95c7-57c68b572238",
			Labels: map[string]string{
				"endpoints.kubernetes.io/managed-by": "endpoint-controller",
			},
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: []corev1.EndpointAddress{
					{
						IP:       podIP,
						NodeName: ptr.To("bunny-01-worker-055ceed2"),
						TargetRef: &corev1.ObjectReference{
							Kind:      "Pod",
							Name:      podName,
							Namespace: namespace,
							UID:       podUID,
						},
					},
				},
				Ports: []corev1.EndpointPort{
					{Name: "petshop-db-bolt", Port: 7687, Protocol: corev1.ProtocolTCP},
					{Name: "petshop-db-http", Port: 7474, Protocol: corev1.ProtocolTCP},
				},
			},
		},
	})

	model.EndpointSlices = append(model.EndpointSlices, &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "petshop-db-service-672nq",
			Namespace: namespace,
			UID:       "a76d8431-236c-4024-9632-087b9f6fc1cf",
			Labels: map[string]string{
				"endpointslice.kubernetes.io/managed-by": "endpointslice-controller.k8s.io",
				"kubernetes.io/service-name":             serviceName,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "v1",
					Kind:               "Service",
					Name:               serviceName,
					UID:                serviceUID,
					BlockOwnerDeletion: ptr.To(true),
					Controller:         ptr.To(true),
				},
			},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{podIP},
				Conditions: discoveryv1.EndpointConditions{
					Ready:       ptr.To(true),
					Serving:     ptr.To(true),
					Terminating: ptr.To(false),
				},
				NodeName: ptr.To("bunny-01-worker-055ceed2"),
				TargetRef: &corev1.ObjectReference{
					Kind:      "Pod",
					Name:      podName,
					Namespace: namespace,
					UID:       podUID,
				},
			},
		},
		Ports: []discoveryv1.EndpointPort{
			{
				Name:     ptr.To("petshop-db-http"),
				Port:     ptr.To(int32(7474)),
				Protocol: ptr.To(corev1.ProtocolTCP),
			},
			{
				Name:     ptr.To("petshop-db-bolt"),
				Port:     ptr.To(int32(7687)),
				Protocol: ptr.To(corev1.ProtocolTCP),
			},
		},
	})

	model.Secrets = append(model.Secrets, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "petshop-motor-secret",
			Namespace: namespace,
			UID:       "motor-orphan-uuid-123",
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"SA-PETSHOP-MOTOR-PASSWORD": []byte("b3JwaGFuZWQtbW90b3Itc2VjcmV0"),
		},
	})

	model.Secrets = append(model.Secrets, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "petshop-web-secret",
			Namespace: namespace,
			UID:       "web-orphan-uuid-456",
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"PETSHOP-WEB-CLIENT-SECRET": []byte("b3JwaGFuZWQtd2ViLXNlY3JldA=="),
		},
	})

	model.Secrets = append(model.Secrets, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "petshop-webservice-secret",
			Namespace: namespace,
			UID:       "webservice-orphan-uuid-789",
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"API_KEY": []byte("b3JwaGFuZWQtd2Vic2VydmljZS1zZWNyZXQ="),
		},
	})
}
