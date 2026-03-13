package mock

import (
	"time"

	kube "github.com/karloie/kompass/pkg/kube"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

func addZookeeperStatefulSet(model *kube.InMemoryModel) {
	namespace := "kafka-system"
	appName := "zookeeper"

	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appName,
			Namespace: namespace,
			UID:       "zk-statefulset-uuid",
			Labels: map[string]string{
				"app.kubernetes.io/name":     appName,
				"app.kubernetes.io/instance": appName,
			},
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: appName,
			Replicas:    int32Ptr(3),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": appName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name":     appName,
						"app.kubernetes.io/instance": appName,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "zookeeper",
							Image: "docker-hub/zookeeper:3.8.3",
							Ports: []corev1.ContainerPort{
								{Name: "client", ContainerPort: 2181, Protocol: corev1.ProtocolTCP},
								{Name: "follower", ContainerPort: 2888, Protocol: corev1.ProtocolTCP},
								{Name: "election", ContainerPort: 3888, Protocol: corev1.ProtocolTCP},
							},
							Env: []corev1.EnvVar{
								{Name: "ZOO_MY_ID", Value: "1"},
								{Name: "ZOO_SERVERS", Value: "server.1=zookeeper-0.zookeeper:2888:3888 server.2=zookeeper-1.zookeeper:2888:3888 server.3=zookeeper-2.zookeeper:2888:3888"},
							},
						},
					},
				},
			},
		},
		Status: appsv1.StatefulSetStatus{
			Replicas:        3,
			ReadyReplicas:   3,
			CurrentReplicas: 3,
			UpdatedReplicas: 3,
		},
	}
	model.StatefulSets = append(model.StatefulSets, statefulSet)

	for i := 0; i < 3; i++ {
		podName := "zookeeper-" + string(rune('0'+i))
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: namespace,
				UID:       types.UID("zk-pod-" + string(rune('0'+i)) + "-uuid"),
				Labels: map[string]string{
					"app.kubernetes.io/name":     appName,
					"app.kubernetes.io/instance": appName,
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "apps/v1",
						Kind:       "StatefulSet",
						Name:       appName,
						UID:        statefulSet.UID,
						Controller: ptr.To(true),
					},
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "zookeeper",
						Image: "docker-hub/zookeeper:3.8.3",
						Ports: []corev1.ContainerPort{
							{Name: "client", ContainerPort: 2181, Protocol: corev1.ProtocolTCP},
							{Name: "follower", ContainerPort: 2888, Protocol: corev1.ProtocolTCP},
							{Name: "election", ContainerPort: 3888, Protocol: corev1.ProtocolTCP},
						},
					},
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				PodIP: "10.244.10." + string(rune('1'+i)),
				ContainerStatuses: []corev1.ContainerStatus{
					{
						Name:    "zookeeper",
						Ready:   true,
						Started: ptr.To(true),
						Image:   "docker-hub/zookeeper:3.8.3",
						ImageID: "docker-hub/zookeeper:3.8.3@sha256:mock-zookeeper-" + string(rune('0'+i)),
						State: corev1.ContainerState{
							Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(time.Date(2026, time.March, 12, 10, 0, 0, 0, time.UTC))},
						},
					},
				},
			},
		}
		model.Pods = append(model.Pods, pod)
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appName,
			Namespace: namespace,
			UID:       "zk-svc-uuid",
			Labels: map[string]string{
				"app.kubernetes.io/name":     appName,
				"app.kubernetes.io/instance": appName,
			},
		},
		Spec: corev1.ServiceSpec{
			Type:      corev1.ServiceTypeClusterIP,
			ClusterIP: "None",
			Selector: map[string]string{
				"app.kubernetes.io/name": appName,
			},
			Ports: []corev1.ServicePort{
				{Name: "client", Port: 2181, TargetPort: intstr.FromInt(2181), Protocol: corev1.ProtocolTCP},
				{Name: "follower", Port: 2888, TargetPort: intstr.FromInt(2888), Protocol: corev1.ProtocolTCP},
				{Name: "election", Port: 3888, TargetPort: intstr.FromInt(3888), Protocol: corev1.ProtocolTCP},
			},
		},
	}
	model.Services = append(model.Services, service)

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appName,
			Namespace: namespace,
			UID:       "zk-sa-uuid",
		},
	}
	model.ServiceAccounts = append(model.ServiceAccounts, sa)
}

func int32Ptr(i int) *int32 {
	i32 := int32(i)
	return &i32
}
