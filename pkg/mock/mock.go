package mock

import (
	kube "github.com/karloie/kompass/pkg/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GenerateMock() *kube.InMemoryModel {
	model := kube.NewModel()

	model.Namespaces = append(model.Namespaces, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "petshop",
		},
	})

	model.Namespaces = append(model.Namespaces, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "management",
		},
	})

	model.Namespaces = append(model.Namespaces, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kafka-system",
		},
	})

	addZookeeperStatefulSet(model)

	addDBApp(model)

	addMotorApp(model)
	addWebApp(model)
	addWebServiceApp(model)
	addMotorCronWorkloads(model)
	addNodeAgentDaemonSet(model)
	addLedgerStatefulSet(model)
	addCoverageSignalResources(model)
	addSecretProviderClasses(model)

	addKafkaApp(model)

	addGatewayResources(model)

	addStandaloneResources(model)

	return model
}

func addSecretProviderClasses(model *kube.InMemoryModel) {
	model.SecretProviderClasses = append(model.SecretProviderClasses,
		map[string]any{
			"apiVersion": "secrets-store.csi.x-k8s.io/v1",
			"kind":       "SecretProviderClass",
			"metadata": map[string]any{
				"name":      "petshop-tennant-petshopvault",
				"namespace": "petshop",
			},
			"spec": map[string]any{
				"provider": "azure",
				"secretObjects": []any{
					map[string]any{"secretName": "petshop-tennant-secrets"},
				},
			},
		},
		map[string]any{
			"apiVersion": "secrets-store.csi.x-k8s.io/v1",
			"kind":       "SecretProviderClass",
			"metadata": map[string]any{
				"name":      "petshop-frontend-girls-petshopvault",
				"namespace": "petshop",
			},
			"spec": map[string]any{
				"provider": "azure",
				"secretObjects": []any{
					map[string]any{"secretName": "petshop-frontend-girls-secrets"},
				},
			},
		},
		map[string]any{
			"apiVersion": "secrets-store.csi.x-k8s.io/v1",
			"kind":       "SecretProviderClass",
			"metadata": map[string]any{
				"name":      "petshop-db-vault",
				"namespace": "petshop",
			},
			"spec": map[string]any{
				"provider": "azure",
				"secretObjects": []any{
					map[string]any{"secretName": "petshop-db-secrets"},
				},
			},
		},
		map[string]any{
			"apiVersion": "secrets-store.csi.x-k8s.io/v1",
			"kind":       "SecretProviderClass",
			"metadata": map[string]any{
				"name":      "petshop-kafka-petshopvault",
				"namespace": "petshop",
			},
			"spec": map[string]any{
				"provider": "azure",
				"secretObjects": []any{
					map[string]any{"secretName": "petshop-kafka-secrets"},
				},
			},
		},
	)
}
