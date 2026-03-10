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

	addKafkaApp(model)

	addGatewayResources(model)

	addStandaloneResources(model)

	return model
}
