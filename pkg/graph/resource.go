package graph

import kube "github.com/karloie/kompass/pkg/kube"

// handlers maps resource types to their graph inference functions.
var handlers = map[string]func(edges *[]kube.ResourceEdge, item *kube.Resource, nodes *map[string]kube.Resource, provider kube.Provider) error{
	"certificate":                    inferCertificate,
	"ciliumclusterwidenetworkpolicy": inferCiliumClusterwideNetworkPolicy,
	"ciliumnetworkpolicy":            inferCiliumNetworkPolicy,
	"clusterissuer":                  inferClusterIssuer,
	"clusterrole":                    inferClusterRole,
	"clusterrolebinding": func(e *[]kube.ResourceEdge, i *kube.Resource, n *map[string]kube.Resource, p kube.Provider) error {
		return inferBinding(e, i, n, p, "clusterrolebinding", "clusterrole")
	},
	"configmap":               inferSimpleNode("configmap"),
	"cronjob":                 inferWorkload("cronjob"),
	"csidriver":               inferCSIDriver,
	"csinode":                 inferCSINode,
	"daemonset":               inferWorkload("daemonset"),
	"deployment":              inferWorkload("deployment"),
	"endpoints":               inferEndpoints,
	"endpointslice":           inferEndpointSlices,
	"gateway":                 inferGateway,
	"horizontalpodautoscaler": inferHorizontalPodAutoscaler,
	"httproute":               inferHTTPRoute,
	"ingress":                 inferIngress,
	"ingressclass":            inferIngressClass,
	"issuer":                  inferIssuer,
	"job":                     inferWorkload("job"),
	"limitrange":              inferSimpleNode("limitrange"),
	"networkpolicy":           inferNetworkPolicy,
	"persistentvolume":        inferPersistentVolume,
	"persistentvolumeclaim":   inferPersistentVolumeClaim,
	"pod":                     inferPod,
	"poddisruptionbudget":     inferPodDisruptionBudget,
	"replicaset":              inferReplicaSet,
	"resourcequota":           inferSimpleNode("resourcequota"),
	"role":                    inferRole,
	"rolebinding": func(e *[]kube.ResourceEdge, i *kube.Resource, n *map[string]kube.Resource, p kube.Provider) error {
		return inferBinding(e, i, n, p, "rolebinding", "role")
	},
	"secret":              inferSimpleNode("secret"),
	"secretproviderclass": inferSimpleNode("secretproviderclass"),
	"service":             inferService,
	"serviceaccount":      inferServiceAccount,
	"statefulset":         inferWorkload("statefulset"),
	"storageclass":        inferStorageClass,
	"volumeattachment":    inferVolumeAttachment,
}
