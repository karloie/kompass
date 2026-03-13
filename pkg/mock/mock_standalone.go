package mock

import (
	kube "github.com/karloie/kompass/pkg/kube"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func addStandaloneResources(model *kube.InMemoryModel) {

	model.CiliumNetworkPolicies = append(model.CiliumNetworkPolicies, map[string]any{
		"apiVersion": "cilium.io/v2",
		"kind":       "CiliumNetworkPolicy",
		"metadata": map[string]any{
			"name":      "allow-external-egress",
			"namespace": "petshop",
			"uid":       "cilium-external-egress-uuid",
		},
		"spec": map[string]any{
			"egress": []any{
				map[string]any{
					"toEntities": []any{"cluster"},
				},
			},
		},
	})

	model.CiliumNetworkPolicies = append(model.CiliumNetworkPolicies, map[string]any{
		"apiVersion": "cilium.io/v2",
		"kind":       "CiliumNetworkPolicy",
		"metadata": map[string]any{
			"name":      "allow-observability",
			"namespace": "petshop",
			"uid":       "cilium-observability-uuid",
		},
		"spec": map[string]any{
			"egress": []any{
				map[string]any{
					"toEndpoints": []any{
						map[string]any{
							"matchLabels": map[string]any{
								"app.kubernetes.io/name": "prometheus",
							},
						},
					},
				},
			},
		},
	})

	model.CiliumNetworkPolicies = append(model.CiliumNetworkPolicies, map[string]any{
		"apiVersion": "cilium.io/v2",
		"kind":       "CiliumNetworkPolicy",
		"metadata": map[string]any{
			"name":      "allow-tracing",
			"namespace": "petshop",
			"uid":       "cilium-tracing-uuid",
		},
		"spec": map[string]any{
			"endpointSelector": map[string]any{
				"matchLabels": map[string]any{
					"tracing": "enabled",
				},
			},
			"egress": []any{
				map[string]any{
					"toEndpoints": []any{
						map[string]any{
							"matchLabels": map[string]any{
								"app.kubernetes.io/name": "tempo",
							},
						},
					},
				},
			},
		},
	})

	model.ConfigMaps = append(model.ConfigMaps, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-root-ca.crt",
			Namespace: "petshop",
			UID:       "configmap-root-ca-uuid",
		},
		Data: map[string]string{
			"ca.crt": "-----BEGIN CERTIFICATE-----\nMIIDDDCCAfSgAwIBAgIBATANBgkqhkiG9w0BAQsFADAw...\n-----END CERTIFICATE-----",
		},
	})

	model.ServiceAccounts = append(model.ServiceAccounts, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "petshop",
			UID:       "serviceaccount-default-uuid",
		},
	})

	model.Secrets = append(model.Secrets, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "docker-registry-credentials",
			Namespace: "petshop",
			UID:       "secret-docker-registry-uuid",
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			".dockerconfigjson": []byte(`{"auths":{"cr.petshop.com":{"username":"_json_key","password":"...","auth":"..."}}}`),
		},
	})

	model.Secrets = append(model.Secrets, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tls-wildcard-cert",
			Namespace: "petshop",
			UID:       "secret-wildcard-tls-uuid",
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": []byte("-----BEGIN CERTIFICATE-----\nMIIFXTCCA0WgAwIBAgISA...\n-----END CERTIFICATE-----"),
			"tls.key": []byte("-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEF...\n-----END PRIVATE KEY-----"),
			"ca.crt":  []byte("-----BEGIN CERTIFICATE-----\nMIIFFjCCAv6gAwIBAgIRAJ...\n-----END CERTIFICATE-----"),
		},
	})

	minAvailable := intstr.FromInt(2)
	model.PodDisruptionBudgets = append(model.PodDisruptionBudgets, &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "petshop-tennant-pdb",
			Namespace: "petshop",
			UID:       "pdb-motor-uuid",
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MinAvailable: &minAvailable,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": "petshop-tennant",
				},
			},
		},
		Status: policyv1.PodDisruptionBudgetStatus{
			CurrentHealthy: 3,
			DesiredHealthy: 2,
		},
	})

	model.ResourceQuotas = append(model.ResourceQuotas, &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "petshop-quota",
			Namespace: "petshop",
			UID:       "resourcequota-petshop-uuid",
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{
				"requests.cpu":    resource.MustParse("10"),
				"requests.memory": resource.MustParse("20Gi"),
				"limits.cpu":      resource.MustParse("20"),
				"limits.memory":   resource.MustParse("40Gi"),
				"pods":            resource.MustParse("50"),
			},
		},
		Status: corev1.ResourceQuotaStatus{
			Used: corev1.ResourceList{
				"requests.cpu":    resource.MustParse("5"),
				"requests.memory": resource.MustParse("10Gi"),
				"limits.cpu":      resource.MustParse("10"),
				"limits.memory":   resource.MustParse("20Gi"),
				"pods":            resource.MustParse("15"),
			},
		},
	})

	model.LimitRanges = append(model.LimitRanges, &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "petshop-limits",
			Namespace: "petshop",
			UID:       "limitrange-petshop-uuid",
		},
		Spec: corev1.LimitRangeSpec{
			Limits: []corev1.LimitRangeItem{
				{
					Type: corev1.LimitTypeContainer,
					Max: corev1.ResourceList{
						"cpu":    resource.MustParse("2"),
						"memory": resource.MustParse("4Gi"),
					},
					Min: corev1.ResourceList{
						"cpu":    resource.MustParse("100m"),
						"memory": resource.MustParse("128Mi"),
					},
					Default: corev1.ResourceList{
						"cpu":    resource.MustParse("500m"),
						"memory": resource.MustParse("512Mi"),
					},
					DefaultRequest: corev1.ResourceList{
						"cpu":    resource.MustParse("250m"),
						"memory": resource.MustParse("256Mi"),
					},
				},
			},
		},
	})

	isDefault := true
	model.IngressClasses = append(model.IngressClasses, &networkingv1.IngressClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nginx",
			UID:  "ingressclass-nginx-uuid",
			Annotations: map[string]string{
				"ingressclass.kubernetes.io/is-default-class": "true",
			},
		},
		Spec: networkingv1.IngressClassSpec{
			Controller: "k8s.io/ingress-nginx",
			Parameters: &networkingv1.IngressClassParametersReference{
				Kind: "IngressParameters",
				Name: "nginx-params",
			},
		},
	})
	_ = isDefault

}
