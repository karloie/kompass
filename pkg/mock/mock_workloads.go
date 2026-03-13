package mock

import (
	"time"

	kube "github.com/karloie/kompass/pkg/kube"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

func addMotorCronWorkloads(model *kube.InMemoryModel) {
	namespace := "petshop"
	cronJobName := "petshop-job"
	jobName := "petshop-job-29557000"
	podName := jobName + "-h7k2m"
	cronUID := types.UID("cron-9fa47b10")
	jobUID := types.UID("job-9fa47b10")
	podUID := types.UID("pod-9fa47b10")
	saUID := types.UID("sa-9fa47b10")

	model.ServiceAccounts = append(model.ServiceAccounts, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "petshop-tennant-cronjob",
			Namespace: namespace,
			UID:       saUID,
			Labels: map[string]string{
				"app.kubernetes.io/name":     "petshop-tennant",
				"app.kubernetes.io/instance": "petshop-tennant",
			},
		},
	})

	model.CronJobs = append(model.CronJobs, &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cronJobName,
			Namespace: namespace,
			UID:       cronUID,
			Labels: map[string]string{
				"app.kubernetes.io/name":     "petshop-tennant",
				"app.kubernetes.io/instance": "petshop-tennant",
				"hit":                        "its-a-sin",
			},
		},
		Spec: batchv1.CronJobSpec{
			Schedule:                   "*/30 * * * *",
			Suspend:                    ptr.To(false),
			ConcurrencyPolicy:          batchv1.ForbidConcurrent,
			SuccessfulJobsHistoryLimit: ptr.To(int32(3)),
			FailedJobsHistoryLimit:     ptr.To(int32(2)),
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					BackoffLimit: ptr.To(int32(1)),
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app.kubernetes.io/name": "petshop-job",
							},
						},
						Spec: corev1.PodSpec{
							RestartPolicy:      corev1.RestartPolicyNever,
							ServiceAccountName: "petshop-tennant-cronjob",
							Containers: []corev1.Container{{
								Name:  "reconcile",
								Image: "petshop/petshop-tennant:tr-7.0.7",
								Args:  []string{"--mode", "reconcile"},
							}},
						},
					},
				},
			},
		},
		Status: batchv1.CronJobStatus{
			LastScheduleTime: ptr.To(metav1.NewTime(time.Date(2026, time.March, 12, 10, 30, 0, 0, time.UTC))),
		},
	})

	model.Jobs = append(model.Jobs, &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
			UID:       jobUID,
			Labels: map[string]string{
				"app.kubernetes.io/name": "petshop-job",
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "batch/v1",
				Kind:       "CronJob",
				Name:       cronJobName,
				UID:        cronUID,
				Controller: ptr.To(true),
			}},
		},
		Spec: batchv1.JobSpec{
			Completions: ptr.To(int32(1)),
			Parallelism: ptr.To(int32(1)),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyNever,
					ServiceAccountName: "petshop-tennant-cronjob",
					Containers: []corev1.Container{{
						Name:  "reconcile",
						Image: "petshop/petshop-tennant:tr-7.0.7",
					}},
				},
			},
		},
		Status: batchv1.JobStatus{
			Succeeded: 1,
			Conditions: []batchv1.JobCondition{{
				Type:   batchv1.JobComplete,
				Status: corev1.ConditionTrue,
				Reason: "Completed",
			}},
			StartTime:      ptr.To(metav1.NewTime(time.Date(2026, time.March, 12, 10, 30, 5, 0, time.UTC))),
			CompletionTime: ptr.To(metav1.NewTime(time.Date(2026, time.March, 12, 10, 30, 18, 0, time.UTC))),
		},
	})

	model.Pods = append(model.Pods, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			UID:       podUID,
			Labels: map[string]string{
				"app.kubernetes.io/name": "petshop-job",
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "batch/v1",
				Kind:       "Job",
				Name:       jobName,
				UID:        jobUID,
				Controller: ptr.To(true),
			}},
		},
		Spec: corev1.PodSpec{
			NodeName:           "psb-01-worker-055ceed5",
			RestartPolicy:      corev1.RestartPolicyNever,
			ServiceAccountName: "petshop-tennant-cronjob",
			Containers: []corev1.Container{{
				Name:  "reconcile",
				Image: "petshop/petshop-tennant:tr-7.0.7",
			}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodSucceeded,
			PodIP: "10.244.9.252",
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:    "reconcile",
				Ready:   false,
				Started: ptr.To(false),
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode:    0,
						Reason:      "Completed",
						FinishedAt:  metav1.NewTime(time.Date(2026, time.March, 12, 10, 30, 18, 0, time.UTC)),
						StartedAt:   metav1.NewTime(time.Date(2026, time.March, 12, 10, 30, 5, 0, time.UTC)),
						ContainerID: "containerd://reconcile-29557000",
					},
				},
			}},
		},
	})
}

func addNodeAgentDaemonSet(model *kube.InMemoryModel) {
	namespace := "petshop"
	name := "petshop-node-agent"
	daemonUID := types.UID("ds-7cb32211")

	model.DaemonSets = append(model.DaemonSets, &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       daemonUID,
			Labels: map[string]string{
				"app.kubernetes.io/name": name,
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
				"app.kubernetes.io/name": name,
			}},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{Type: appsv1.RollingUpdateDaemonSetStrategyType},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app.kubernetes.io/name": name}},
				Spec: corev1.PodSpec{Containers: []corev1.Container{{
					Name:  "agent",
					Image: "petshop/node-agent:tb-3.0.3",
					Ports: []corev1.ContainerPort{{Name: "metrics", ContainerPort: 9102, Protocol: corev1.ProtocolTCP}},
				}}},
			},
		},
		Status: appsv1.DaemonSetStatus{
			DesiredNumberScheduled: 3,
			CurrentNumberScheduled: 2,
			NumberReady:            1,
			UpdatedNumberScheduled: 2,
			NumberUnavailable:      2,
		},
	})

	model.Pods = append(model.Pods, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-g7z4r",
			Namespace: namespace,
			UID:       types.UID("pod-7cb32211"),
			Labels: map[string]string{
				"app.kubernetes.io/name": name,
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "apps/v1",
				Kind:       "DaemonSet",
				Name:       name,
				UID:        daemonUID,
				Controller: ptr.To(true),
			}},
		},
		Spec: corev1.PodSpec{
			NodeName: "psb-02-worker-12345678",
			Containers: []corev1.Container{{
				Name:  "agent",
				Image: "petshop/node-agent:tb-3.0.3",
			}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.244.10.42",
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:    "agent",
				Ready:   true,
				Started: ptr.To(true),
				State:   corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(time.Date(2026, time.March, 12, 9, 45, 0, 0, time.UTC))}},
			}},
		},
	})
}

func addLedgerStatefulSet(model *kube.InMemoryModel) {
	namespace := "petshop"
	name := "petshop-ledger"
	statefulSetUID := types.UID("sts-41be98c0")

	model.StatefulSets = append(model.StatefulSets, &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       statefulSetUID,
			Labels: map[string]string{
				"app.kubernetes.io/name": name,
			},
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: name,
			Replicas:    ptr.To(int32(2)),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
				"app.kubernetes.io/name": name,
			}},
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{Type: appsv1.RollingUpdateStatefulSetStrategyType},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app.kubernetes.io/name": name}},
				Spec: corev1.PodSpec{Containers: []corev1.Container{{
					Name:         "ledger",
					Image:        "petshop/ledger:tr-8.0.8",
					Ports:        []corev1.ContainerPort{{Name: "http", ContainerPort: 8080, Protocol: corev1.ProtocolTCP}},
					VolumeMounts: []corev1.VolumeMount{{Name: "ledger-data", MountPath: "/var/lib/ledger"}},
				}}},
			},
		},
		Status: appsv1.StatefulSetStatus{
			Replicas:        2,
			ReadyReplicas:   1,
			CurrentReplicas: 2,
			UpdatedReplicas: 1,
		},
	})

	model.Services = append(model.Services, &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       types.UID("svc-41be98c0"),
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
			Selector: map[string]string{
				"app.kubernetes.io/name": name,
			},
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Port:       8080,
				TargetPort: intstr.FromInt(8080),
				Protocol:   corev1.ProtocolTCP,
			}},
		},
	})

	model.Pods = append(model.Pods, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-0",
			Namespace: namespace,
			UID:       types.UID("pod-41be98c0"),
			Labels: map[string]string{
				"app.kubernetes.io/name": name,
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "apps/v1",
				Kind:       "StatefulSet",
				Name:       name,
				UID:        statefulSetUID,
				Controller: ptr.To(true),
			}},
		},
		Spec: corev1.PodSpec{
			NodeName: "psb-01-worker-055ceed3",
			Containers: []corev1.Container{{
				Name:  "ledger",
				Image: "petshop/ledger:tr-8.0.8",
			}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.244.9.253",
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:    "ledger",
				Ready:   true,
				Started: ptr.To(true),
				State:   corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(time.Date(2026, time.March, 12, 8, 0, 0, 0, time.UTC))}},
			}},
		},
	})
}

func addCoverageSignalResources(model *kube.InMemoryModel) {
	namespace := "petshop"

	model.HorizontalPodAutoscalers = append(model.HorizontalPodAutoscalers, &autoscalingv1.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "petshop-boys-motor-hpa",
			Namespace: namespace,
			UID:       types.UID("hpa-b0y5a11"),
			Labels: map[string]string{
				"hit": "west-end-girls",
			},
		},
		Spec: autoscalingv1.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv1.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "petshop-tennant",
			},
			MinReplicas: ptr.To(int32(2)),
			MaxReplicas: 6,
		},
		Status: autoscalingv1.HorizontalPodAutoscalerStatus{
			CurrentReplicas: 3,
			DesiredReplicas: 4,
		},
	})

	buildNode := func(name, uid, hit string) *corev1.Node {
		return &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				UID:  types.UID(uid),
				Labels: map[string]string{
					"hit": hit,
				},
			},
			Status: corev1.NodeStatus{
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("8"),
					corev1.ResourceMemory: resource.MustParse("32Gi"),
				},
				NodeInfo: corev1.NodeSystemInfo{
					OSImage:        "Ubuntu 24.04 LTS",
					KernelVersion:  "6.8.0-40-generic",
					KubeletVersion: "v1.32.3",
				},
				Conditions: []corev1.NodeCondition{{
					Type:   corev1.NodeReady,
					Status: corev1.ConditionTrue,
				}},
			},
		}
	}

	model.Nodes = append(model.Nodes,
		buildNode("psb-boys-01", "node-b0y5a11", "go-west"),
		buildNode("psb-01-worker-055ceed2", "node-055ceed2", "left-to-my-own-devices"),
		buildNode("psb-01-worker-055ceed3", "node-055ceed3", "rent"),
		buildNode("psb-01-worker-055ceed4", "node-055ceed4", "suburbia"),
		buildNode("psb-01-worker-055ceed5", "node-055ceed5", "domino-dancing"),
		buildNode("psb-02-worker-12345678", "node-12345678", "heart"),
		buildNode("psb-03-worker-99999999", "node-99999999", "being-boring"),
	)

	model.Services = append(model.Services, &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "petshop-lowe",
			Namespace: namespace,
			UID:       types.UID("svc-b0y5a11"),
			Labels: map[string]string{
				"app.kubernetes.io/name": "petshop-frontend-girls",
				"tribute":                "boys",
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			Selector: map[string]string{
				"app.kubernetes.io/name":     "petshop-frontend-girls",
				"app.kubernetes.io/instance": "petshop-frontend-girls",
			},
			Ports: []corev1.ServicePort{{
				Name:       "https",
				Port:       443,
				TargetPort: intstr.FromInt(8080),
				Protocol:   corev1.ProtocolTCP,
			}},
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{
					{Hostname: "boys.petshop.com"},
					{Hostname: "west-end-girls.petshop.com"},
					{Hostname: "its-a-sin.petshop.com"},
					{Hostname: "always-on-my-mind.petshop.com"},
					{Hostname: "go-west.petshop.com"},
					{Hostname: "opportunities.petshop.com"},
				},
			},
		},
	})

	probePodName := "petshop-node-agent-west-end-girls"
	failedProbePodName := "petshop-node-agent-its-a-sin"
	daemonSetUID := types.UID("ds-7cb32211")

	probeContainer := corev1.Container{
		Name:  "agent",
		Image: "petshop/node-agent:tb-3.0.3",
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: "/livez", Port: intstr.FromInt(8080)}},
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: "/readyz", Port: intstr.FromInt(8080)}},
		},
		StartupProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{Exec: &corev1.ExecAction{Command: []string{"/bin/sh", "-c", "test -f /tmp/started"}}},
		},
	}

	model.Pods = append(model.Pods, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      probePodName,
			Namespace: namespace,
			UID:       types.UID("pod-b0y5good"),
			Labels: map[string]string{
				"app.kubernetes.io/name": "petshop-node-agent",
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "apps/v1",
				Kind:       "DaemonSet",
				Name:       "petshop-node-agent",
				UID:        daemonSetUID,
				Controller: ptr.To(true),
			}},
		},
		Spec: corev1.PodSpec{NodeName: "psb-boys-01", Containers: []corev1.Container{probeContainer}},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.244.10.43",
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:    "agent",
				Ready:   true,
				Started: ptr.To(true),
				State:   corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(time.Date(2026, time.March, 12, 9, 50, 0, 0, time.UTC))}},
			}},
		},
	})

	model.Pods = append(model.Pods, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      failedProbePodName,
			Namespace: namespace,
			UID:       types.UID("pod-b0y5bad"),
			Labels: map[string]string{
				"app.kubernetes.io/name": "petshop-node-agent",
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "apps/v1",
				Kind:       "DaemonSet",
				Name:       "petshop-node-agent",
				UID:        daemonSetUID,
				Controller: ptr.To(true),
			}},
		},
		Spec: corev1.PodSpec{NodeName: "psb-02-worker-12345678", Containers: []corev1.Container{probeContainer}},
		Status: corev1.PodStatus{
			Phase: corev1.PodFailed,
			PodIP: "10.244.10.44",
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:    "agent",
				Ready:   false,
				Started: ptr.To(false),
				State:   corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "Error", ExitCode: 137}},
			}},
		},
	})
}
