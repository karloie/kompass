package mock

import (
	"time"

	kube "github.com/karloie/kompass/pkg/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

func addMotorApp(model *kube.InMemoryModel) {
	addAppWithSpecs(model, appSpec{
		name:   "petshop-tennant",
		podUID: "130a1d4e-056c-4dd3-b5c2-d9d8fc67caa8",
		rsHash: "5689f8488b",
		podIP:  "10.244.9.239",
		image:  "petshop/petshop-tennant:tr-7.0.7",
		envVars: []corev1.EnvVar{
			{Name: "BACKEND_BOYS_URL", Value: "http://petshop-backend-boys:8080"},
			{Name: "LDAP_SECRET", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "petshop-tennant-secrets"}, Key: "PBS-TENNANT-PASSWORD"}}},
			{Name: "LDAP_USER", Value: "petshop-tennant-sa"},
			{Name: "LOGGING_LEVEL", Value: "INFO"},
			{Name: "NEO4J_PASSWORD", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "petshop-tennant-secrets"}, Key: "PBS-DATABASE-PASSWORD"}}},
			{Name: "NEO4J_URL", Value: "bolt://petshop-db-service:7687"},
			{Name: "NEO4J_USERNAME", Value: "neo4j"},
		},
		volumes: []corev1.Volume{
			{Name: "tmp", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory}}},
			{Name: "petshopvault", VolumeSource: corev1.VolumeSource{CSI: &corev1.CSIVolumeSource{Driver: "secrets-store.csi.k8s.io", ReadOnly: ptr.To(true), VolumeAttributes: map[string]string{"secretProviderClass": "petshop-tennant-petshopvault"}}}},
		},
		volumeMounts: []corev1.VolumeMount{
			{Name: "tmp", MountPath: "/tmp"},
			{Name: "petshopvault", MountPath: "/mnt/secrets", ReadOnly: true},
		},
	})
}

func addWebApp(model *kube.InMemoryModel) {
	addAppWithSpecs(model, appSpec{
		name:   "petshop-frontend-girls",
		podUID: "8c2b3d4e-567f-89ab-cdef-123456789012",
		rsHash: "598696998b",
		podIP:  "10.244.9.240",
		image:  "petshop/petshop-frontend-girls:tr-6.0.6",
		envVars: []corev1.EnvVar{
			{Name: "LOG_LEVEL", Value: "info"},
			{Name: "NEO4J_PASSWORD", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "petshop-frontend-girls-secrets"}, Key: "PBS-DATABASE-PASSWORD"}}},
			{Name: "NEO4J_URI", Value: "bolt://petshop-db-service:7687"},
			{Name: "NEO4J_USERNAME", Value: "neo4j"},
			{Name: "OIDC_CLIENT_ID", Value: "petshop-frontend-girls"},
			{Name: "OIDC_CLIENT_SECRET", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "petshop-frontend-girls-secrets"}, Key: "PBS-OIDC-SECRET"}}},
			{Name: "OIDC_ISSUER_URL", Value: "https://auth.petshop.com/oauth2"},
			{Name: "OIDC_REDIRECT_URL", Value: "https://frontend-girls.petshop.com/auth/callback"},
			{Name: "REQUIRE_SECURE_CONNECTION", Value: "true"},
		},
		volumes: []corev1.Volume{
			{Name: "tmp", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory}}},
			{Name: "petshopvault", VolumeSource: corev1.VolumeSource{CSI: &corev1.CSIVolumeSource{Driver: "secrets-store.csi.k8s.io", ReadOnly: ptr.To(true), VolumeAttributes: map[string]string{"secretProviderClass": "petshop-frontend-girls-petshopvault"}}}},
		},
		volumeMounts: []corev1.VolumeMount{
			{Name: "tmp", MountPath: "/tmp"},
			{Name: "petshopvault", MountPath: "/mnt/secrets", ReadOnly: true},
		},
	})

	namespace := "petshop"
	replicaSetName := "petshop-frontend-girls-598696998b"
	replicaSetUID := types.UID("rs-8c2b3d4e")

	labels := map[string]string{
		"app.kubernetes.io/instance":   "petshop-frontend-girls",
		"app.kubernetes.io/name":       "petshop-frontend-girls",
		"app.kubernetes.io/managed-by": "Helm",
		"helm.sh/chart":                "petshop-app-helm-chart-0.18.1",
		"pod-template-hash":            "598696998b",
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      replicaSetName + "-v58bh",
			Namespace: namespace,
			UID:       types.UID("8c2b3d4e-567f-89ab-cdef-999999999999"),
			Labels:    labels,
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
			ServiceAccountName: "petshop-frontend-girls",
			ImagePullSecrets: []corev1.LocalObjectReference{
				{Name: "docker-registry-credentials"},
			},
			Containers: []corev1.Container{
				{
					Name:            "app",
					Image:           "petshop/petshop-frontend-girls:tr-6.0.6",
					ImagePullPolicy: corev1.PullAlways,
					Env: []corev1.EnvVar{
						{Name: "LOG_LEVEL", Value: "info"},
						{Name: "NEO4J_PASSWORD", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "petshop-frontend-girls-secrets"}, Key: "PBS-DATABASE-PASSWORD"}}},
						{Name: "NEO4J_URI", Value: "bolt://petshop-db-service:7687"},
						{Name: "NEO4J_USERNAME", Value: "neo4j"},
					},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "tmp", MountPath: "/tmp"},
						{Name: "petshopvault", MountPath: "/mnt/secrets", ReadOnly: true},
					},
					Ports: []corev1.ContainerPort{
						{Name: "http", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
					},
				},
			},
			Volumes: []corev1.Volume{
				{Name: "tmp", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory}}},
				{Name: "petshopvault", VolumeSource: corev1.VolumeSource{CSI: &corev1.CSIVolumeSource{Driver: "secrets-store.csi.k8s.io", ReadOnly: ptr.To(true), VolumeAttributes: map[string]string{"secretProviderClass": "petshop-frontend-girls-petshopvault"}}}},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.244.9.250",
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:    "app",
					Ready:   true,
					Started: ptr.To(true),
					Image:   "petshop/petshop-frontend-girls:tr-6.0.6",
					ImageID: "petshop/petshop-frontend-girls:tr-6.0.6@sha256:shahaha",
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
}

func addWebServiceApp(model *kube.InMemoryModel) {
	addAppWithSpecs(model, appSpec{
		name:   "petshop-backend-boys",
		podUID: "9d3c4e5f-678a-9bcd-ef01-234567890123",
		rsHash: "7d9b7b4cd",
		podIP:  "10.244.9.241",
		image:  "petshop/petshop-backend-boys:dj-2.0.2",
		envVars: []corev1.EnvVar{
			{Name: "LOGLEVEL", Value: "INFO"},
		},
		volumes: []corev1.Volume{
			{Name: "tmp", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory}}},
		},
		volumeMounts: []corev1.VolumeMount{
			{Name: "tmp", MountPath: "/tmp"},
		},
	})
}
