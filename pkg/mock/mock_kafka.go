package mock

import (
	kube "github.com/karloie/kompass/pkg/kube"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

func addKafkaApp(model *kube.InMemoryModel) {
	namespace := "petshop"

	model.ConfigMaps = append(model.ConfigMaps, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kafka-server-config",
			Namespace: namespace,
			UID:       "kafka-configmap-uuid-001",
		},
		Data: map[string]string{
			"server.properties": "broker.id=1\nlog.dirs=/var/log/kafka\nnum.partitions=3",
			"log4j.properties":  "log4j.rootLogger=INFO, stdout",
			"log-level":         "INFO",
		},
		BinaryData: map[string][]byte{
			"truststore.jks": []byte("BINARY_JKS_DATA_HERE"),
		},
	})

	model.ConfigMaps = append(model.ConfigMaps, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kafka-runtime-config",
			Namespace: namespace,
			UID:       "kafka-configmap-uuid-002",
		},
		Data: map[string]string{
			"retention-hours": "168",
			"compression":     "lz4",
		},
	})

	model.Secrets = append(model.Secrets, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kafka-tls-certs",
			Namespace: namespace,
			UID:       "kafka-secret-uuid-001",
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": []byte("LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURYVENDQWtXZ0F3SUJBZ0lVRXhh"),
			"tls.key": []byte("LS0tLS1CRUdJTiBQUklWQVRFIEtFWS0tLS0tCk1JSUV2UUlCQURBTkJna3Foa2lHOXcw"),
			"ca.crt":  []byte("LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURRekNDQWl1Z0F3SUJBZ0lVUnlS"),
		},
	})

	addAppWithSpecs(model, appSpec{
		name:   "petshop-kafka",
		podUID: "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		rsHash: "6b7c8d9e0f",
		podIP:  "10.244.9.242",
		image:  "docker-hub/confluentinc/cp-kafka:7.5.0",

		ports: []corev1.ContainerPort{
			{Name: "ssl", ContainerPort: 9093, Protocol: corev1.ProtocolTCP, HostPort: 9093},
			{Name: "http", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
			{Name: "metrics", ContainerPort: 7071, Protocol: corev1.ProtocolTCP},
			{Name: "admin-udp", ContainerPort: 9095, Protocol: corev1.ProtocolUDP},
			{Name: "sctp-port", ContainerPort: 9096, Protocol: corev1.ProtocolSCTP, HostIP: "0.0.0.0"},
		},

		hostNetwork: false,
		dnsPolicy:   corev1.DNSClusterFirst,
		hostname:    "kafka-broker-1",
		subdomain:   "kafka",

		envVars: []corev1.EnvVar{

			{Name: "KAFKA_BROKER_ID", Value: "1"},

			{Name: "KAFKA_LOG_LEVEL", ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "kafka-server-config"},
					Key:                  "log-level",
				},
			}},

			{Name: "KAFKA_RETENTION_HOURS", ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "kafka-runtime-config"},
					Key:                  "retention-hours",
				},
			}},

			{Name: "POD_NAME", ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			}},

			{Name: "MEMORY_LIMIT", ValueFrom: &corev1.EnvVarSource{
				ResourceFieldRef: &corev1.ResourceFieldSelector{
					ContainerName: "app",
					Resource:      "limits.memory",
					Divisor:       resource.MustParse("1Mi"),
				},
			}},
		},

		volumes: []corev1.Volume{

			{
				Name: "tmp",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						Medium: corev1.StorageMediumMemory,
					},
				},
			},

			{
				Name: "kafka-server-config-vol",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "kafka-server-config",
						},
						DefaultMode: ptr.To(int32(0644)),
					},
				},
			},

			{
				Name: "kafka-tls-vol",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  "kafka-tls-certs",
						DefaultMode: ptr.To(int32(0400)),
					},
				},
			},

			{
				Name: "petshopvault",
				VolumeSource: corev1.VolumeSource{
					CSI: &corev1.CSIVolumeSource{
						Driver:   "secrets-store.csi.k8s.io",
						ReadOnly: ptr.To(true),
						VolumeAttributes: map[string]string{
							"secretProviderClass": "petshop-kafka-petshopvault",
							"provider":            "azure",
						},
					},
				},
			},

			{
				Name: "host-log-dir",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/var/log/kafka",
						Type: ptr.To(corev1.HostPathDirectoryOrCreate),
					},
				},
			},

			{
				Name: "projected-combined",
				VolumeSource: corev1.VolumeSource{
					Projected: &corev1.ProjectedVolumeSource{
						DefaultMode: ptr.To(int32(0644)),
						Sources: []corev1.VolumeProjection{
							{
								Secret: &corev1.SecretProjection{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "kafka-tls-certs",
									},
									Items: []corev1.KeyToPath{
										{Key: "ca.crt", Path: "ca.crt"},
									},
								},
							},
							{
								ConfigMap: &corev1.ConfigMapProjection{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "kafka-server-config",
									},
									Items: []corev1.KeyToPath{
										{Key: "server.properties", Path: "server.properties"},
									},
								},
							},
							{
								ConfigMap: &corev1.ConfigMapProjection{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "kafka-runtime-config",
									},
									Items: []corev1.KeyToPath{
										{Key: "retention-hours", Path: "runtime/retention-hours"},
										{Key: "compression", Path: "runtime/compression"},
									},
								},
							},
							{
								DownwardAPI: &corev1.DownwardAPIProjection{
									Items: []corev1.DownwardAPIVolumeFile{
										{
											Path: "namespace",
											FieldRef: &corev1.ObjectFieldSelector{
												FieldPath: "metadata.namespace",
											},
										},
									},
								},
							},
						},
					},
				},
			},

			{
				Name: "podinfo",
				VolumeSource: corev1.VolumeSource{
					DownwardAPI: &corev1.DownwardAPIVolumeSource{
						DefaultMode: ptr.To(int32(0644)),
						Items: []corev1.DownwardAPIVolumeFile{
							{
								Path: "name",
								FieldRef: &corev1.ObjectFieldSelector{
									FieldPath: "metadata.name",
								},
							},
							{
								Path: "cpu_limit",
								ResourceFieldRef: &corev1.ResourceFieldSelector{
									ContainerName: "app",
									Resource:      "limits.cpu",
									Divisor:       resource.MustParse("1m"),
								},
							},
						},
					},
				},
			},
		},

		volumeMounts: []corev1.VolumeMount{
			{Name: "tmp", MountPath: "/tmp", ReadOnly: false},
			{Name: "kafka-server-config-vol", MountPath: "/etc/kafka/config", ReadOnly: true},
			{Name: "kafka-tls-vol", MountPath: "/etc/kafka/tls", ReadOnly: true},
			{Name: "petshopvault", MountPath: "/run/secrets", ReadOnly: true},
			{Name: "host-log-dir", MountPath: "/kafka-logs", ReadOnly: false},
			{Name: "projected-combined", MountPath: "/etc/kafka/projected", ReadOnly: true},
			{Name: "podinfo", MountPath: "/etc/podinfo", ReadOnly: true},
		},
	})

	// Keep a few stale target refs to model endpoint lag during pod churn.
	for i, eps := range model.EndpointSlices {
		if eps.Name == "petshop-kafka-abcde" {

			model.EndpointSlices[i].Endpoints = append(model.EndpointSlices[i].Endpoints,

				discoveryv1.Endpoint{
					Addresses: []string{"10.244.9.243"},
					Conditions: discoveryv1.EndpointConditions{
						Ready:       ptr.To(true),
						Serving:     ptr.To(true),
						Terminating: ptr.To(false),
					},
					NodeName: ptr.To("psb-01-worker-055ceed3"),
					TargetRef: &corev1.ObjectReference{
						Kind:      "Pod",
						Name:      "petshop-kafka-6b7c8d9e0f-v58bh",
						Namespace: namespace,
						UID:       types.UID("1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6e"),
					},
				},

				discoveryv1.Endpoint{
					Addresses: []string{"10.244.9.244"},
					Conditions: discoveryv1.EndpointConditions{
						Ready:       ptr.To(false),
						Serving:     ptr.To(true),
						Terminating: ptr.To(true),
					},
					NodeName: ptr.To("psb-01-worker-055ceed4"),
					TargetRef: &corev1.ObjectReference{
						Kind:      "Pod",
						Name:      "petshop-kafka-6b7c8d9e0f-zzzzz",
						Namespace: namespace,
						UID:       types.UID("1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6f"),
					},
				},

				discoveryv1.Endpoint{
					Addresses: []string{"10.244.9.245"},
					Conditions: discoveryv1.EndpointConditions{
						Ready:       ptr.To(false),
						Serving:     ptr.To(false),
						Terminating: ptr.To(false),
					},
					NodeName: ptr.To("psb-01-worker-055ceed5"),
					TargetRef: &corev1.ObjectReference{
						Kind:      "Pod",
						Name:      "petshop-kafka-6b7c8d9e0f-aaaaa",
						Namespace: namespace,
						UID:       types.UID("1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c70"),
					},
				},

				discoveryv1.Endpoint{
					Addresses: []string{"10.244.10.100"},
					Conditions: discoveryv1.EndpointConditions{
						Ready:       ptr.To(true),
						Serving:     ptr.To(true),
						Terminating: ptr.To(false),
					},
					Hostname: ptr.To("kafka-0"),
					NodeName: ptr.To("psb-02-worker-12345678"),
					TargetRef: &corev1.ObjectReference{
						Kind:      "Pod",
						Name:      "petshop-kafka-6b7c8d9e0f-bbbbb",
						Namespace: namespace,
						UID:       types.UID("1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c71"),
					},
					Zone: ptr.To("europe-north1-a"),
				},
			)
			break
		}
	}

	for i, ep := range model.Endpoints {
		if ep.Name == "petshop-kafka" && ep.Namespace == namespace {

			if len(model.Endpoints[i].Subsets) > 0 {
				model.Endpoints[i].Subsets[0].Addresses = append(model.Endpoints[i].Subsets[0].Addresses,

					corev1.EndpointAddress{
						IP:       "10.244.9.243",
						NodeName: ptr.To("psb-01-worker-055ceed3"),
						TargetRef: &corev1.ObjectReference{
							Kind:      "Pod",
							Name:      "petshop-kafka-6b7c8d9e0f-v58bh",
							Namespace: namespace,
							UID:       types.UID("1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6e"),
						},
					},

					corev1.EndpointAddress{
						IP:       "10.244.10.100",
						Hostname: "kafka-0",
						NodeName: ptr.To("psb-02-worker-12345678"),
						TargetRef: &corev1.ObjectReference{
							Kind:      "Pod",
							Name:      "petshop-kafka-6b7c8d9e0f-bbbbb",
							Namespace: namespace,
							UID:       types.UID("1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c71"),
						},
					},
				)

				model.Endpoints[i].Subsets[0].NotReadyAddresses = []corev1.EndpointAddress{

					{
						IP:       "10.244.9.244",
						NodeName: ptr.To("psb-01-worker-055ceed4"),
						TargetRef: &corev1.ObjectReference{
							Kind:      "Pod",
							Name:      "petshop-kafka-6b7c8d9e0f-zzzzz",
							Namespace: namespace,
							UID:       types.UID("1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6f"),
						},
					},

					{
						IP:       "10.244.9.245",
						NodeName: ptr.To("psb-01-worker-055ceed5"),
						TargetRef: &corev1.ObjectReference{
							Kind:      "Pod",
							Name:      "petshop-kafka-6b7c8d9e0f-aaaaa",
							Namespace: namespace,
							UID:       types.UID("1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c70"),
						},
					},
				}
			}

			model.Endpoints[i].Subsets = append(model.Endpoints[i].Subsets, corev1.EndpointSubset{
				Addresses: []corev1.EndpointAddress{
					{
						IP:       "10.244.11.50",
						NodeName: ptr.To("psb-03-worker-99999999"),
						TargetRef: &corev1.ObjectReference{
							Kind:      "Pod",
							Name:      "petshop-kafka-6b7c8d9e0f-ccccc",
							Namespace: namespace,
							UID:       types.UID("1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c72"),
						},
					},
				},
				Ports: []corev1.EndpointPort{
					{Name: "kafka-ssl", Port: 9093, Protocol: corev1.ProtocolTCP},
					{Name: "kafka-metrics", Port: 7071, Protocol: corev1.ProtocolTCP},
				},
			})

			break
		}
	}

	model.CiliumNetworkPolicies = append(model.CiliumNetworkPolicies, map[string]any{
		"apiVersion": "cilium.io/v2",
		"kind":       "CiliumNetworkPolicy",
		"metadata": map[string]any{
			"name":      "petshop-kafka",
			"namespace": namespace,
			"uid":       "cilium-kafka-policy-uuid",
		},
		"spec": map[string]any{
			"endpointSelector": map[string]any{
				"matchLabels": map[string]any{
					"app.kubernetes.io/instance": "petshop-kafka",
					"app.kubernetes.io/name":     "petshop-kafka",
				},
			},

			"ingress": []any{

				map[string]any{
					"fromEntities": []any{"cluster"},
					"toPorts": []any{
						map[string]any{
							"ports": []any{
								map[string]any{"port": "9093", "protocol": "TCP"},
								map[string]any{"port": "9094", "protocol": "TCP"},
							},
						},
					},
				},

				map[string]any{
					"fromEndpoints": []any{
						map[string]any{
							"matchLabels": map[string]any{
								"app.kubernetes.io/name": "petshop-frontend-girls",
							},
						},
						map[string]any{
							"matchLabels": map[string]any{
								"app.kubernetes.io/name": "petshop-backend-boys",
							},
						},
					},
					"toPorts": []any{
						map[string]any{
							"ports": []any{
								map[string]any{"port": "8080", "protocol": "TCP"},
							},
						},
					},
				},
			},

			"egress": []any{

				map[string]any{
					"toEndpoints": []any{
						map[string]any{
							"matchLabels": map[string]any{
								"k8s:io.kubernetes.pod.namespace": "kube-system",
								"k8s:k8s-app":                     "kube-dns",
							},
						},
					},
					"toPorts": []any{
						map[string]any{
							"ports": []any{
								map[string]any{"port": "53", "protocol": "UDP"},
							},
						},
					},
				},

				map[string]any{
					"toEndpoints": []any{
						map[string]any{
							"matchLabels": map[string]any{
								"k8s:io.kubernetes.pod.namespace": "kafka-system",
								"k8s:app.kubernetes.io/name":      "zookeeper",
							},
						},
					},
				},

				map[string]any{
					"toEndpoints": []any{
						map[string]any{
							"matchLabels": map[string]any{
								"app.kubernetes.io/name": "schema-registry",
							},
						},
					},
					"toPorts": []any{
						map[string]any{
							"ports": []any{
								map[string]any{"port": "443", "protocol": "TCP"},
							},
						},
					},
				},
			},
			"enableDefaultDeny": map[string]any{
				"egress":  true,
				"ingress": true,
			},
		},
	})

	model.Ingresses = append(model.Ingresses, &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "petshop-kafka-ingress",
			Namespace: namespace,
			UID:       "kafka-ingress-uuid",
			Labels: map[string]string{
				"hit": "its-a-sin",
			},
			Annotations: map[string]string{
				"cert-manager.io/cluster-issuer":               "letsencrypt-prod",
				"nginx.ingress.kubernetes.io/ssl-redirect":     "true",
				"nginx.ingress.kubernetes.io/backend-protocol": "HTTP",
			},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: ptr.To("nginx"),
			TLS: []networkingv1.IngressTLS{
				{
					Hosts:      []string{"kafka.petshop.com", "its-a-sin-kafka.petshop.com"},
					SecretName: "kafka-tls-certs",
				},
			},
			Rules: []networkingv1.IngressRule{
				{
					Host: "kafka.petshop.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: ptr.To(networkingv1.PathTypePrefix),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "petshop-kafka",
											Port: networkingv1.ServiceBackendPort{
												Number: 8080,
											},
										},
									},
								},
							},
						},
					},
				},
				{
					Host: "its-a-sin-kafka.petshop.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: ptr.To(networkingv1.PathTypePrefix),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "petshop-kafka",
											Port: networkingv1.ServiceBackendPort{
												Number: 8080,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		Status: networkingv1.IngressStatus{
			LoadBalancer: networkingv1.IngressLoadBalancerStatus{
				Ingress: []networkingv1.IngressLoadBalancerIngress{
					{
						IP:       "10.0.100.50",
						Hostname: "kafka.petshop.com",
					},
				},
			},
		},
	})

	model.HTTPRoutes = append(model.HTTPRoutes, map[string]any{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "HTTPRoute",
		"metadata": map[string]any{
			"name":      "petshop-kafka",
			"namespace": namespace,
			"uid":       "httproute-kafka-uuid",
			"labels": map[string]any{
				"app.kubernetes.io/instance":   "petshop-kafka",
				"app.kubernetes.io/name":       "petshop-kafka",
				"app.kubernetes.io/managed-by": "Helm",
				"hit":                          "its-a-sin",
			},
		},
		"spec": map[string]any{
			"parentRefs": []any{
				map[string]any{
					"group":     "gateway.networking.k8s.io",
					"kind":      "Gateway",
					"name":      "internal-gateway",
					"namespace": "management",
				},
			},
			"hostnames": []any{"kafka.petshop.com", "its-a-sin-kafka.petshop.com"},
			"rules": []any{
				map[string]any{
					"matches": []any{
						map[string]any{
							"path": map[string]any{
								"type":  "PathPrefix",
								"value": "/",
							},
						},
					},
					"backendRefs": []any{
						map[string]any{
							"name":      "petshop-kafka",
							"namespace": namespace,
							"port":      8080,
							"weight":    100,
						},
					},
				},
			},
		},
		"status": map[string]any{
			"parents": []any{
				map[string]any{
					"parentRef": map[string]any{
						"group":     "gateway.networking.k8s.io",
						"kind":      "Gateway",
						"name":      "internal-gateway",
						"namespace": "management",
					},
					"conditions": []any{
						map[string]any{
							"type":               "Accepted",
							"status":             "True",
							"reason":             "Accepted",
							"lastTransitionTime": "2026-03-05T10:00:00Z",
						},
						map[string]any{
							"type":               "ResolvedRefs",
							"status":             "True",
							"reason":             "ResolvedRefs",
							"lastTransitionTime": "2026-03-05T10:00:00Z",
						},
					},
				},
			},
		},
	})

	model.EndpointSlices = append(model.EndpointSlices, &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kafka-schema-registry-fqdn",
			Namespace: namespace,
			UID:       "schema-registry-epslice-fqdn-uuid",
			Labels: map[string]string{
				"kubernetes.io/service-name": "kafka-schema-registry",
			},
		},
		AddressType: discoveryv1.AddressTypeFQDN,
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{"schema-registry-1.confluent.cloud"},
				Conditions: discoveryv1.EndpointConditions{
					Ready:       ptr.To(true),
					Serving:     ptr.To(true),
					Terminating: ptr.To(false),
				},
			},
			{
				Addresses: []string{"schema-registry-2.confluent.cloud"},
				Conditions: discoveryv1.EndpointConditions{
					Ready:       ptr.To(true),
					Serving:     ptr.To(true),
					Terminating: ptr.To(false),
				},
			},
		},
		Ports: []discoveryv1.EndpointPort{
			{Name: ptr.To("https"), Port: ptr.To(int32(443)), Protocol: ptr.To(corev1.ProtocolTCP)},
		},
	})

	model.Services = append(model.Services, &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kafka-schema-registry",
			Namespace: namespace,
			UID:       "schema-registry-svc-uuid",
			Labels: map[string]string{
				"app.kubernetes.io/name": "schema-registry",
			},
		},
		Spec: corev1.ServiceSpec{
			Type:      corev1.ServiceTypeClusterIP,
			ClusterIP: "None",
			Ports: []corev1.ServicePort{
				{Name: "https", Port: 443, Protocol: corev1.ProtocolTCP},
			},
		},
	})
}
