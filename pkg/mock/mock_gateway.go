package mock

import (
	"time"

	kube "github.com/karloie/kompass/pkg/kube"
)

func addGatewayResources(model *kube.InMemoryModel) {

	model.CiliumNetworkPolicies = append(model.CiliumNetworkPolicies, map[string]any{
		"apiVersion": "cilium.io/v2",
		"kind":       "CiliumNetworkPolicy",
		"metadata": map[string]any{
			"name":      "petshop-db",
			"namespace": "petshop",
			"uid":       "cilium-db-policy-uuid",
		},
		"spec": map[string]any{
			"endpointSelector": map[string]any{
				"matchLabels": map[string]any{
					"app.kubernetes.io/instance": "petshop-db",
					"app.kubernetes.io/name":     "petshop-db",
				},
			},
			"ingress": []any{
				map[string]any{
					"fromEntities": []any{"cluster"},
				},
				map[string]any{
					"fromEndpoints": []any{
						map[string]any{
							"matchLabels": map[string]any{
								"app.kubernetes.io/name": "petshop-tennant",
							},
						},
						map[string]any{
							"matchLabels": map[string]any{
								"app.kubernetes.io/name": "petshop-frontend-girls",
							},
						},
					},
				},
			},
			"enableDefaultDeny": map[string]any{
				"egress":  false,
				"ingress": true,
			},
		},
	})

	model.CiliumNetworkPolicies = append(model.CiliumNetworkPolicies, map[string]any{
		"apiVersion": "cilium.io/v2",
		"kind":       "CiliumNetworkPolicy",
		"metadata": map[string]any{
			"name":      "petshop-tennant",
			"namespace": "petshop",
			"uid":       "cilium-motor-policy-uuid",
		},
		"spec": map[string]any{
			"endpointSelector": map[string]any{
				"matchLabels": map[string]any{
					"app.kubernetes.io/instance": "petshop-tennant",
					"app.kubernetes.io/name":     "petshop-tennant",
				},
			},
			"egress": []any{
				map[string]any{
					"toEndpoints": []any{
						map[string]any{
							"matchLabels": map[string]any{
								"app.kubernetes.io/name": "petshop-backend-boys",
							},
						},
						map[string]any{
							"matchLabels": map[string]any{
								"app.kubernetes.io/name": "petshop-db",
							},
						},
					},
				},
			},
			"ingress": []any{
				map[string]any{
					"fromEntities": []any{"cluster"},
				},
			},
			"enableDefaultDeny": map[string]any{
				"egress":  false,
				"ingress": true,
			},
		},
	})

	model.HTTPRoutes = append(model.HTTPRoutes, map[string]any{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "HTTPRoute",
		"metadata": map[string]any{
			"name":      "petshop-tennant",
			"namespace": "petshop",
			"uid":       "httproute-motor-uuid",
			"labels": map[string]any{
				"app.kubernetes.io/instance":   "petshop-tennant",
				"app.kubernetes.io/name":       "petshop-tennant",
				"app.kubernetes.io/managed-by": "Helm",
				"helm.sh/chart":                "petshop-app-helm-chart-0.18.1",
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
			"rules": []any{
				map[string]any{
					"backendRefs": []any{
						map[string]any{
							"group": "",
							"kind":  "Service",
							"name":  "petshop-tennant",
							"port":  8080,
						},
					},
					"matches": []any{
						map[string]any{
							"path": map[string]any{
								"type":  "PathPrefix",
								"value": "/",
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
			"name":      "petshop-frontend-girls",
			"namespace": "petshop",
			"uid":       "cilium-web-policy-uuid",
		},
		"spec": map[string]any{
			"endpointSelector": map[string]any{
				"matchLabels": map[string]any{
					"app.kubernetes.io/instance": "petshop-frontend-girls",
					"app.kubernetes.io/name":     "petshop-frontend-girls",
				},
			},
			"egress": []any{
				map[string]any{
					"toFQDNs": []any{
						map[string]any{
							"matchName": "am.dev.petshop.com",
						},
					},
				},
				map[string]any{
					"toEndpoints": []any{
						map[string]any{
							"matchLabels": map[string]any{
								"app.kubernetes.io/name": "petshop-db",
							},
						},
					},
				},
			},
			"ingress": []any{
				map[string]any{
					"fromEntities": []any{"cluster"},
				},
			},
			"enableDefaultDeny": map[string]any{
				"egress":  false,
				"ingress": true,
			},
		},
	})

	model.HTTPRoutes = append(model.HTTPRoutes, map[string]any{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "HTTPRoute",
		"metadata": map[string]any{
			"name":      "petshop-frontend-girls",
			"namespace": "petshop",
			"uid":       "httproute-web-uuid",
			"labels": map[string]any{
				"app.kubernetes.io/instance":   "petshop-frontend-girls",
				"app.kubernetes.io/name":       "petshop-frontend-girls",
				"app.kubernetes.io/managed-by": "Helm",
				"helm.sh/chart":                "petshop-app-helm-chart-0.18.1",
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
			"hostnames": []any{"frontend-girls.petshop.com", "frontend-girls.petshop.com"},
			"rules": []any{
				map[string]any{
					"backendRefs": []any{
						map[string]any{
							"group": "",
							"kind":  "Service",
							"name":  "petshop-frontend-girls",
							"port":  8080,
						},
					},
					"matches": []any{
						map[string]any{
							"path": map[string]any{
								"type":  "PathPrefix",
								"value": "/",
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
			"name":      "petshop-backend-boys",
			"namespace": "petshop",
			"uid":       "cilium-webservice-policy-uuid",
		},
		"spec": map[string]any{
			"endpointSelector": map[string]any{
				"matchLabels": map[string]any{
					"app.kubernetes.io/instance": "petshop-backend-boys",
					"app.kubernetes.io/name":     "petshop-backend-boys",
				},
			},
			"egress": []any{
				map[string]any{
					"toFQDNs": []any{
						map[string]any{
							"matchName": "mdc11.petshop.com",
						},
					},
				},
			},
			"ingress": []any{
				map[string]any{
					"fromEntities": []any{"cluster"},
				},
				map[string]any{
					"fromEndpoints": []any{
						map[string]any{
							"matchLabels": map[string]any{
								"app.kubernetes.io/name": "petshop-tennant",
							},
						},
						map[string]any{
							"matchLabels": map[string]any{
								"app.kubernetes.io/name": "petshop-frontend-girls",
							},
						},
					},
				},
			},
			"enableDefaultDeny": map[string]any{
				"egress":  false,
				"ingress": true,
			},
		},
	})

	model.HTTPRoutes = append(model.HTTPRoutes, map[string]any{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "HTTPRoute",
		"metadata": map[string]any{
			"name":      "petshop-backend-boys",
			"namespace": "petshop",
			"uid":       "httproute-webservice-uuid",
			"labels": map[string]any{
				"app.kubernetes.io/instance":   "petshop-backend-boys",
				"app.kubernetes.io/name":       "petshop-backend-boys",
				"app.kubernetes.io/managed-by": "Helm",
				"helm.sh/chart":                "petshop-app-helm-chart-0.18.1",
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
			"hostnames": []any{"backend-boys.petshop.com", "backend-boys.petshop.com"},
			"rules": []any{
				map[string]any{
					"backendRefs": []any{
						map[string]any{
							"group": "",
							"kind":  "Service",
							"name":  "petshop-backend-boys",
							"port":  8080,
						},
					},
					"matches": []any{
						map[string]any{
							"path": map[string]any{
								"type":  "PathPrefix",
								"value": "/",
							},
						},
					},
				},
			},
		},
	})

	model.Gateways = append(model.Gateways, map[string]any{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "Gateway",
		"metadata": map[string]any{
			"name":      "internal-gateway",
			"namespace": "management",
			"uid":       "2a3ff77e-266c-4218-9709-012f6f980de4",
			"labels": map[string]any{
				"argocd.argoproj.io/instance": "gateway-api-config-psb",
			},
		},
		"spec": map[string]any{
			"gatewayClassName": "cilium",
			"infrastructure": map[string]any{
				"annotations": map[string]any{
					"lbipam.cilium.io/ips": "10.200.80.9",
				},
			},
			"listeners": []any{
				map[string]any{
					"name":     "https",
					"port":     443,
					"protocol": "HTTPS",
					"allowedRoutes": map[string]any{
						"namespaces": map[string]any{
							"from": "All",
						},
					},
					"tls": map[string]any{
						"mode": "Terminate",
						"certificateRefs": []any{
							map[string]any{
								"group": "",
								"kind":  "Secret",
								"name":  "default-gateway-certificate",
							},
						},
					},
				},
			},
		},
		"status": map[string]any{
			"addresses": []any{
				map[string]any{
					"type":  "IPAddress",
					"value": "10.200.80.9",
				},
			},
			"conditions": []any{
				map[string]any{
					"type":               "Programmed",
					"status":             "True",
					"reason":             "Programmed",
					"message":            "Gateway Programmed",
					"observedGeneration": 2,
				},
			},
			"listeners": []any{
				map[string]any{
					"name":           "https",
					"attachedRoutes": 27,
					"conditions": []any{
						map[string]any{
							"type":   "Programmed",
							"status": "True",
							"reason": "Programmed",
						},
					},
				},
			},
		},
	})

	model.Certificates = append(model.Certificates, map[string]any{
		"apiVersion": "cert-manager.io/v1",
		"kind":       "Certificate",
		"metadata": map[string]any{
			"name":      "default-gateway-certificate",
			"namespace": "management",
			"uid":       "a9c9616c-be8a-4c03-ac17-e1d99dd4a020",
			"labels": map[string]any{
				"argocd.argoproj.io/instance": "gateway-api-config-psb",
			},
		},
		"spec": map[string]any{
			"commonName": "*.mock.no",
			"dnsNames": []any{
				"*.mock.no",
				"*.mock.com",
			},
			"duration":    "2160h",
			"renewBefore": "360h",
			"secretName":  "default-gateway-certificate",
			"issuerRef": map[string]any{
				"kind": "ClusterIssuer",
				"name": "letsencrypt-prod",
			},
		},
		"status": map[string]any{
			"notAfter": time.Now().Add(15 * 24 * time.Hour).Format(time.RFC3339),
			"conditions": []any{
				map[string]any{
					"type":    "Ready",
					"status":  "True",
					"reason":  "Ready",
					"message": "Certificate is up to date and has not expired",
				},
			},
		},
	})

	model.Certificates = append(model.Certificates, map[string]any{
		"apiVersion": "cert-manager.io/v1",
		"kind":       "Certificate",
		"metadata": map[string]any{
			"name":      "expired-cert",
			"namespace": "management",
			"uid":       "b1c1d1e1-1234-5678-9abc-def012345678",
		},
		"spec": map[string]any{
			"commonName": "expired.mock.no",
			"dnsNames": []any{
				"expired.mock.no",
			},
			"secretName": "expired-cert-secret",
		},
		"status": map[string]any{
			"notAfter": time.Now().Add(-5 * 24 * time.Hour).Format(time.RFC3339),
			"conditions": []any{
				map[string]any{
					"type":    "Ready",
					"status":  "False",
					"reason":  "Expired",
					"message": "Certificate has expired",
				},
			},
		},
	})

	model.Certificates = append(model.Certificates, map[string]any{
		"apiVersion": "cert-manager.io/v1",
		"kind":       "Certificate",
		"metadata": map[string]any{
			"name":      "long-lived-cert",
			"namespace": "management",
			"uid":       "c2d2e2f2-5678-9abc-def0-123456789abc",
		},
		"spec": map[string]any{
			"commonName": "long-lived.mock.no",
			"dnsNames": []any{
				"long-lived.mock.no",
			},
			"secretName": "long-lived-cert-secret",
			"issuerRef": map[string]any{
				"kind": "Issuer",
				"name": "management-ca",
			},
		},
		"status": map[string]any{
			"notAfter": time.Now().Add(60 * 24 * time.Hour).Format(time.RFC3339),
			"conditions": []any{
				map[string]any{
					"type":    "Ready",
					"status":  "True",
					"reason":  "Ready",
					"message": "Certificate is up to date and has not expired",
				},
			},
		},
	})

	model.Issuers = append(model.Issuers, map[string]any{
		"apiVersion": "cert-manager.io/v1",
		"kind":       "Issuer",
		"metadata": map[string]any{
			"name":      "management-ca",
			"namespace": "management",
			"uid":       "d3e3f3a3-6789-abcd-ef01-23456789abcd",
		},
		"spec": map[string]any{
			"ca": map[string]any{
				"secretName": "management-ca-keypair",
			},
		},
	})

	model.ClusterIssuers = append(model.ClusterIssuers, map[string]any{
		"apiVersion": "cert-manager.io/v1",
		"kind":       "ClusterIssuer",
		"metadata": map[string]any{
			"name": "letsencrypt-prod",
			"uid":  "12345678-1234-1234-1234-123456789abc",
		},
		"spec": map[string]any{
			"acme": map[string]any{
				"server": "https://acme-v02.api.letsencrypt.org/directory",
				"email":  "boys@petshop.com",
				"privateKeySecretRef": map[string]any{
					"name": "letsencrypt-prod-key",
				},
				"solvers": []any{
					map[string]any{
						"http01": map[string]any{
							"ingress": map[string]any{
								"class": "nginx",
							},
						},
					},
				},
			},
		},
	})
}
