package graph

import kube "github.com/karloie/kompass/pkg/kube"

type ResourceType struct {
	Emoji        string
	Loader       kube.ResourceLoader
	Handler      func(edges *[]kube.ResourceEdge, item *kube.Resource, nodes *map[string]kube.Resource, provider kube.Kube) error
	LeafChildren []string
}

var ResourceTypes = func() map[string]ResourceType {
	m := map[string]ResourceType{

		"certificate":                    {"📜", kube.LoadCertificate, inferCertificate, nil},
		"ciliumclusterwidenetworkpolicy": {"🌍", kube.LoadCiliumClusterwideNetworkPolicy, inferCiliumClusterwideNetworkPolicy, []string{"pod"}},
		"ciliumnetworkpolicy":            {"📜", kube.LoadCiliumNetworkPolicy, inferCiliumNetworkPolicy, []string{"service", "pod"}},
		"clusterissuer":                  {"🗝️", kube.LoadClusterIssuer, inferClusterIssuer, nil},
		"clusterrole":                    {"🪪", kube.GetLoader("clusterrole"), inferClusterRole, nil},
		"clusterrolebinding": {"🔗", kube.GetLoader("clusterrolebinding"), func(e *[]kube.ResourceEdge, i *kube.Resource, n *map[string]kube.Resource, p kube.Kube) error {
			return inferBinding(e, i, n, p, "clusterrolebinding", "clusterrole")
		}, []string{"clusterrole", "serviceaccount"}},
		"configmap":               {"📝", kube.GetLoader("configmap"), inferSimpleNode("configmap"), nil},
		"cronjob":                 {"🚀", kube.GetLoader("cronjob"), inferWorkload("cronjob"), []string{"job"}},
		"csidriver":               {"🗄️", kube.GetLoader("csidriver"), inferCSIDriver, []string{"csinode"}},
		"csinode":                 {"🔩", kube.GetLoader("csinode"), inferCSINode, nil},
		"daemonset":               {"🚀", kube.GetLoader("daemonset"), inferWorkload("daemonset"), nil},
		"deployment":              {"🚀", kube.GetLoader("deployment"), inferWorkload("deployment"), nil},
		"endpoints":               {"📍", kube.GetLoader("endpoints"), inferEndpoints, []string{"service"}},
		"endpointslice":           {"📍", kube.GetLoader("endpointslice"), inferEndpointSlices, []string{"service", "pod"}},
		"gateway":                 {"🚺", kube.LoadGateway, inferGateway, nil},
		"horizontalpodautoscaler": {"🧬", kube.GetLoader("horizontalpodautoscaler"), inferHorizontalPodAutoscaler, nil},
		"httproute":               {"🔄", kube.LoadHTTPRoute, inferHTTPRoute, []string{"service", "gateway"}},
		"ingress":                 {"👉", kube.GetLoader("ingress"), inferIngress, []string{"service", "certificate"}},
		"ingressclass":            {"🏷️", kube.GetLoader("ingressclass"), inferIngressClass, nil},
		"issuer":                  {"🔑", kube.LoadIssuer, inferIssuer, []string{"secret", "serviceaccount"}},
		"job":                     {"🎮", kube.LoadJob, inferWorkload("job"), nil},
		"limitrange":              {"📏", kube.GetLoader("limitrange"), inferSimpleNode("limitrange"), nil},
		"networkpolicy":           {"🧱", kube.LoadNetworkPolicy, inferNetworkPolicy, []string{"pod"}},
		"node":                    {"🖥️", kube.LoadNode, nil, nil},
		"persistentvolume":        {"💿", kube.GetLoader("persistentvolume"), inferPersistentVolume, []string{"storageclass", "volumeattachment"}},
		"persistentvolumeclaim":   {"📀", kube.GetLoader("persistentvolumeclaim"), inferPersistentVolumeClaim, []string{"persistentvolume"}},
		"pod":                     {"🫛", kube.LoadPod, inferPod, []string{"serviceaccount", "configmap", "secret", "persistentvolumeclaim", "service"}},
		"poddisruptionbudget":     {"🛡️", kube.GetLoader("poddisruptionbudget"), inferPodDisruptionBudget, nil},
		"replicaset":              {"🎮", kube.GetLoader("replicaset"), inferReplicaSet, nil},
		"resourcequota":           {"📊", kube.GetLoader("resourcequota"), inferSimpleNode("resourcequota"), nil},
		"role":                    {"🪪", kube.GetLoader("role"), inferRole, nil},
		"rolebinding": {"🔗", kube.GetLoader("rolebinding"), func(e *[]kube.ResourceEdge, i *kube.Resource, n *map[string]kube.Resource, p kube.Kube) error {
			return inferBinding(e, i, n, p, "rolebinding", "role")
		}, []string{"role", "serviceaccount"}},
		"secret":           {"🔒", kube.LoadSecret, inferSimpleNode("secret"), nil},
		"service":          {"🖥️", kube.LoadService, inferService, []string{"pod"}},
		"serviceaccount":   {"👤", kube.GetLoader("serviceaccount"), inferServiceAccount, []string{"pod", "rolebinding", "clusterrolebinding"}},
		"statefulset":      {"🚀", kube.GetLoader("statefulset"), inferWorkload("statefulset"), []string{"persistentvolumeclaim"}},
		"storageclass":     {"💽", kube.GetLoader("storageclass"), inferStorageClass, []string{"persistentvolume"}},
		"volumeattachment": {"💿", kube.GetLoader("volumeattachment"), inferVolumeAttachment, []string{"persistentvolume"}},

		"accessmodes":                {"🔓", nil, nil, nil},
		"address":                    {"🔌", nil, nil, nil},
		"args":                       {"🗯️", nil, nil, nil},
		"capacity":                   {"📊", nil, nil, nil},
		"certificatecondition":       {"✅", nil, nil, nil},
		"certificateusage":           {"📋", nil, nil, nil},
		"certificatesigningrequest":  {"🎫", nil, nil, nil},
		"cnp-cw-egress":              {"👈", nil, nil, []string{"service"}},
		"cnp-cw-ingress":             {"👉", nil, nil, nil},
		"cnp-egress":                 {"👈", nil, nil, []string{"service"}},
		"cnp-ingress":                {"👉", nil, nil, nil},
		"command":                    {"⚡", nil, nil, nil},
		"container":                  {"🧊", nil, nil, nil},
		"dnsnames":                   {"🌐", nil, nil, nil},
		"dnspolicy":                  {"🌐", nil, nil, nil},
		"egressrule":                 {"📤", nil, nil, nil},
		"enabledefaultdeny":          {"🚫", nil, nil, nil},
		"endpoint":                   {"🔌", nil, nil, nil},
		"endpointselector":           {"🎯", nil, nil, nil},
		"env":                        {"💬", nil, nil, nil},
		"envars":                     {"🔤", nil, nil, nil},
		"envfrom-configmap":          {"📝", nil, nil, nil},
		"envfrom-secret":             {"🔐", nil, nil, nil},
		"finalizers":                 {"🔒", nil, nil, nil},
		"flowschema":                 {"📄", nil, nil, nil},
		"fromendpoint":               {"📥", nil, nil, nil},
		"fromentities":               {"🌍", nil, nil, nil},
		"hostipc":                    {"🔧", nil, nil, nil},
		"hostnetwork":                {"🌐", nil, nil, nil},
		"hostpid":                    {"🔧", nil, nil, nil},
		"httproute-ref":              {"🔄", nil, nil, nil},
		"image":                      {"🐋", nil, nil, nil},
		"ingress-route":              {"🔄", nil, nil, nil},
		"ingressrule":                {"📥", nil, nil, nil},
		"issuerref":                  {"🔑", nil, nil, nil},
		"label":                      {"🏷️", nil, nil, nil},
		"labels":                     {"🏷️", nil, nil, nil},
		"lifecycle":                  {"♻️", nil, nil, nil},
		"livenessprobe":              {"💓", nil, nil, nil},
		"mount":                      {"📁", nil, nil, nil},
		"mounts":                     {"📂", nil, nil, nil},
		"namespace":                  {"🗂️", nil, nil, nil},
		"netpol-egress":              {"👈", nil, nil, []string{"service"}},
		"netpol-ingress":             {"👉", nil, nil, nil},
		"networking":                 {"🔀", nil, nil, nil},
		"nodeselector":               {"🎯", nil, nil, nil},
		"notreadyaddress":            {"📵", nil, nil, nil},
		"phase":                      {"📍", nil, nil, nil},
		"pod-ref":                    {"🫛", nil, nil, nil},
		"podsecuritycontext":         {"🛡️", nil, nil, nil},
		"port":                       {"⇄", nil, nil, nil},
		"ports":                      {"🔌", nil, nil, nil},
		"priorityclass":              {"⭐", nil, nil, nil},
		"prioritylevelconfiguration": {"📄", nil, nil, nil},
		"qosclass":                   {"📊", nil, nil, nil},
		"readinessprobe":             {"✅", nil, nil, nil},
		"resources":                  {"💾", nil, nil, nil},
		"restartpolicy":              {"🔄", nil, nil, nil},
		"runtimeclass":               {"🏃", nil, nil, nil},
		"schedule":                   {"⏰", nil, nil, nil},
		"securitycontext":            {"🛡️", nil, nil, nil},
		"secretkey":                  {"🔑", nil, nil, nil},
		"secretkeys":                 {"🗝️", nil, nil, nil},
		"secretname":                 {"🔒", nil, nil, nil},
		"secrettype":                 {"📋", nil, nil, nil},
		"startupprobe":               {"▶️", nil, nil, nil},
		"storage":                    {"💾", nil, nil, nil},
		"subset":                     {"🔗", nil, nil, nil},
		"tls-cert":                   {"🔒", nil, nil, nil},
		"tls-secret":                 {"🔐", nil, nil, nil},
		"toendpoint":                 {"📤", nil, nil, nil},
		"toentities":                 {"🌍", nil, nil, nil},
		"tofqdn":                     {"🌐", nil, nil, nil},
		"toport":                     {"🔌", nil, nil, nil},
		"toleration":                 {"🤝", nil, nil, nil},
		"tolerations":                {"🎯", nil, nil, nil},
		"topologyspreadconstraints":  {"📐", nil, nil, nil},
		"volumename":                 {"💿", nil, nil, nil},
	}
	return m
}()

func IsResourceType(s string) bool {
	_, exists := ResourceTypes[s]
	return exists
}

func GetResourceEmoji(resourceType string) string {
	if rt, exists := ResourceTypes[resourceType]; exists && rt.Emoji != "" {
		return rt.Emoji
	}
	return "📄"
}
