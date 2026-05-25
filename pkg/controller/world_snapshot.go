package controller

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"fixora/pkg/finops"
	"fixora/pkg/metrics"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
)

type WorldSnapshot struct {
	GeneratedAt time.Time
	Cluster     string
	Workloads   map[string]*WorldWorkload
	Pods        map[string]*WorldPod
	Services    map[string]*WorldService
	Ingresses   map[string]*WorldIngress
	Nodes       map[string]*WorldNode
	Edges       []WorldEdge
	Traffic     []WorldTrafficEdge
}

type WorldWorkload struct {
	ID                    string
	Cluster               string
	Namespace             string
	Kind                  string
	Name                  string
	Selector              string
	Status                string
	Desired               int32
	Ready                 int32
	Available             int32
	Updated               int32
	Pods                  []string
	Services              []string
	Ingresses             []string
	NodeNames             []string
	Labels                map[string]string
	Annotations           map[string]string
	CPURequestedCores     float64
	MemoryRequestedBytes  float64
	CPURequestedByNode    map[string]float64
	MemoryRequestedByNode map[string]float64
}

type WorldPod struct {
	ID           string
	Cluster      string
	Namespace    string
	Name         string
	Phase        string
	Reason       string
	NodeName     string
	IP           string
	Ready        bool
	RestartCount int32
	Containers   []string
	Labels       map[string]string
	WorkloadID   string
}

type WorldService struct {
	ID          string
	Cluster     string
	Namespace   string
	Name        string
	Type        string
	ClusterIP   string
	Selector    map[string]string
	Ports       []string
	PodIDs      []string
	WorkloadIDs []string
	IngressIDs  []string
}

type WorldIngress struct {
	ID          string
	Cluster     string
	Namespace   string
	Name        string
	Hosts       []string
	ServiceIDs  []string
	WorkloadIDs []string
}

type WorldNode struct {
	ID           string
	Name         string
	ProviderID   string
	Vendor       string
	Region       string
	Zone         string
	InstanceType string
	Ready        bool
	Labels       map[string]string
}

type WorldEdge struct {
	From string
	To   string
	Kind string
}

type WorldTrafficEdge struct {
	From              string
	To                string
	RequestsPerSecond float64
}

func (c *Controller) BuildWorldSnapshot(ctx context.Context) (*WorldSnapshot, []v1.Pod) {
	cluster := c.dashboardEnvironment(ctx)
	world := &WorldSnapshot{
		GeneratedAt: time.Now(),
		Cluster:     cluster,
		Workloads:   map[string]*WorldWorkload{},
		Pods:        map[string]*WorldPod{},
		Services:    map[string]*WorldService{},
		Ingresses:   map[string]*WorldIngress{},
		Nodes:       map[string]*WorldNode{},
	}
	if c.clientset == nil {
		return world, nil
	}

	edgeSeen := map[string]bool{}
	addEdge := func(from, to, kind string) {
		if from == "" || to == "" || from == to {
			return
		}
		key := from + "\x00" + to + "\x00" + kind
		if edgeSeen[key] {
			return
		}
		edgeSeen[key] = true
		world.Edges = append(world.Edges, WorldEdge{From: from, To: to, Kind: kind})
	}

	if nodes, err := c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{Limit: 500}); err == nil {
		for _, node := range nodes.Items {
			n := worldNodeFromKubernetes(node)
			world.Nodes[n.ID] = n
		}
	}

	c.addControllerWorkloads(ctx, world, cluster)

	var allPods []v1.Pod
	pods, err := c.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err == nil {
		allPods = pods.Items
		for i := range allPods {
			pod := &allPods[i]
			p := worldPodFromKubernetes(cluster, *pod)
			identity := c.workloadIdentityForPod(ctx, pod)
			w := ensureWorldWorkload(world, cluster, pod.Namespace, identity.Kind, identity.Name)
			if w.Selector == "" {
				w.Selector = identity.Selector
			}
			p.WorkloadID = w.ID
			world.Pods[p.ID] = p
			w.Pods = appendUnique(w.Pods, p.ID)
			cpuReq, memReq := podRequestedResources(*pod)
			w.CPURequestedCores += cpuReq
			w.MemoryRequestedBytes += memReq
			if p.NodeName != "" {
				w.NodeNames = appendUnique(w.NodeNames, p.NodeName)
				if w.CPURequestedByNode == nil {
					w.CPURequestedByNode = map[string]float64{}
				}
				if w.MemoryRequestedByNode == nil {
					w.MemoryRequestedByNode = map[string]float64{}
				}
				w.CPURequestedByNode[p.NodeName] += cpuReq
				w.MemoryRequestedByNode[p.NodeName] += memReq
				nodeID := worldNodeID(p.NodeName)
				addEdge(p.ID, nodeID, "scheduled_on")
			}
			addEdge(w.ID, p.ID, "owns")
		}
	}

	c.addWorldServices(ctx, world, cluster, addEdge)
	c.addWorldIngresses(ctx, world, cluster, addEdge)
	c.addWorldTraffic(ctx, world, addEdge)

	for _, workload := range world.Workloads {
		sort.Strings(workload.Pods)
		sort.Strings(workload.Services)
		sort.Strings(workload.Ingresses)
		sort.Strings(workload.NodeNames)
	}
	return world, allPods
}

func (c *Controller) addWorldTraffic(ctx context.Context, world *WorldSnapshot, addEdge func(string, string, string)) {
	provider, ok := c.promClient.(metrics.TrafficGraphProvider)
	if !ok {
		return
	}
	edges, err := provider.GetTrafficEdges(5*time.Minute, 0.01)
	if err != nil {
		return
	}
	for _, edge := range edges {
		src := findWorldWorkloadByName(world, edge.SourceNamespace, edge.SourceWorkload)
		dst := findWorldWorkloadByName(world, edge.DestinationNamespace, edge.DestinationWorkload)
		if src == nil || dst == nil {
			continue
		}
		world.Traffic = append(world.Traffic, WorldTrafficEdge{
			From:              src.ID,
			To:                dst.ID,
			RequestsPerSecond: edge.RequestsPerSecond,
		})
		addEdge(src.ID, dst.ID, "traffic")
	}
}

func (c *Controller) addControllerWorkloads(ctx context.Context, world *WorldSnapshot, cluster string) {
	if deployments, err := c.clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{}); err == nil {
		for _, deploy := range deployments.Items {
			w := ensureWorldWorkload(world, cluster, deploy.Namespace, "Deployment", deploy.Name)
			applyDeploymentToWorldWorkload(w, deploy)
		}
	}
	if statefulSets, err := c.clientset.AppsV1().StatefulSets("").List(ctx, metav1.ListOptions{}); err == nil {
		for _, sts := range statefulSets.Items {
			w := ensureWorldWorkload(world, cluster, sts.Namespace, "StatefulSet", sts.Name)
			applyStatefulSetToWorldWorkload(w, sts)
		}
	}
	if daemonSets, err := c.clientset.AppsV1().DaemonSets("").List(ctx, metav1.ListOptions{}); err == nil {
		for _, ds := range daemonSets.Items {
			w := ensureWorldWorkload(world, cluster, ds.Namespace, "DaemonSet", ds.Name)
			applyDaemonSetToWorldWorkload(w, ds)
		}
	}
	if jobs, err := c.clientset.BatchV1().Jobs("").List(ctx, metav1.ListOptions{}); err == nil {
		for _, job := range jobs.Items {
			w := ensureWorldWorkload(world, cluster, job.Namespace, "Job", job.Name)
			applyJobToWorldWorkload(w, job)
		}
	}
	if cronJobs, err := c.clientset.BatchV1().CronJobs("").List(ctx, metav1.ListOptions{}); err == nil {
		for _, cronJob := range cronJobs.Items {
			w := ensureWorldWorkload(world, cluster, cronJob.Namespace, "CronJob", cronJob.Name)
			w.Status = "schedule=" + cronJob.Spec.Schedule
			w.Labels = cloneStringMap(cronJob.Labels)
			w.Annotations = cloneStringMap(cronJob.Annotations)
		}
	}
}

func (c *Controller) addWorldServices(ctx context.Context, world *WorldSnapshot, cluster string, addEdge func(string, string, string)) {
	services, err := c.clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return
	}

	// Index pods by namespace for faster lookup
	podsByNS := make(map[string][]*WorldPod)
	for _, pod := range world.Pods {
		podsByNS[pod.Namespace] = append(podsByNS[pod.Namespace], pod)
	}

	for _, svc := range services.Items {
		s := worldServiceFromKubernetes(cluster, svc)
		world.Services[s.ID] = s
		if len(svc.Spec.Selector) == 0 {
			continue
		}
		selector := k8slabels.SelectorFromSet(k8slabels.Set(svc.Spec.Selector))

		// Only check pods in the same namespace to avoid O(Services * TotalPods)
		for _, pod := range podsByNS[svc.Namespace] {
			if !selector.Matches(k8slabels.Set(pod.Labels)) {
				continue
			}
			s.PodIDs = appendUnique(s.PodIDs, pod.ID)
			if pod.WorkloadID != "" {
				s.WorkloadIDs = appendUnique(s.WorkloadIDs, pod.WorkloadID)
				if workload := world.Workloads[pod.WorkloadID]; workload != nil {
					workload.Services = appendUnique(workload.Services, s.ID)
				}
				addEdge(pod.WorkloadID, s.ID, "selected_by")
			}
			addEdge(s.ID, pod.ID, "routes_to")
		}
	}
}

func (c *Controller) addWorldIngresses(ctx context.Context, world *WorldSnapshot, cluster string, addEdge func(string, string, string)) {
	ingresses, err := c.clientset.NetworkingV1().Ingresses("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return
	}
	for _, ingress := range ingresses.Items {
		in := worldIngressFromKubernetes(cluster, ingress)
		world.Ingresses[in.ID] = in
		for _, serviceName := range ingressServiceNames(ingress) {
			serviceID := worldID(cluster, ingress.Namespace, "Service", serviceName)
			service := world.Services[serviceID]
			if service == nil {
				continue
			}
			in.ServiceIDs = appendUnique(in.ServiceIDs, serviceID)
			service.IngressIDs = appendUnique(service.IngressIDs, in.ID)
			addEdge(serviceID, in.ID, "exposed_by")
			for _, workloadID := range service.WorkloadIDs {
				in.WorkloadIDs = appendUnique(in.WorkloadIDs, workloadID)
				if workload := world.Workloads[workloadID]; workload != nil {
					workload.Ingresses = appendUnique(workload.Ingresses, in.ID)
				}
			}
		}
		sort.Strings(in.Hosts)
		sort.Strings(in.ServiceIDs)
		sort.Strings(in.WorkloadIDs)
	}
}

func ensureWorldWorkload(world *WorldSnapshot, cluster, namespace, kind, name string) *WorldWorkload {
	kind = firstNonEmpty(strings.TrimSpace(kind), "Pod")
	name = strings.TrimSpace(name)
	id := worldID(cluster, namespace, kind, name)
	if existing := world.Workloads[id]; existing != nil {
		return existing
	}
	workload := &WorldWorkload{
		ID:        id,
		Cluster:   cluster,
		Namespace: namespace,
		Kind:      kind,
		Name:      name,
		Status:    "unknown",
	}
	world.Workloads[id] = workload
	return workload
}

func findWorldWorkloadByName(world *WorldSnapshot, namespace, name string) *WorldWorkload {
	if world == nil || name == "" {
		return nil
	}
	var fallback *WorldWorkload
	for _, workload := range world.Workloads {
		if workload.Namespace != namespace || workload.Name != name {
			continue
		}
		if workload.Kind != "Pod" {
			return workload
		}
		fallback = workload
	}
	return fallback
}

func worldPodFromKubernetes(cluster string, pod v1.Pod) *WorldPod {
	containers := make([]string, 0, len(pod.Spec.InitContainers)+len(pod.Spec.Containers))
	for _, c := range pod.Spec.InitContainers {
		containers = append(containers, "init:"+c.Name)
	}
	for _, c := range pod.Spec.Containers {
		containers = append(containers, c.Name)
	}
	var restarts int32
	for _, status := range pod.Status.InitContainerStatuses {
		restarts += status.RestartCount
	}
	for _, status := range pod.Status.ContainerStatuses {
		restarts += status.RestartCount
	}
	return &WorldPod{
		ID:           worldID(cluster, pod.Namespace, "Pod", pod.Name),
		Cluster:      cluster,
		Namespace:    pod.Namespace,
		Name:         pod.Name,
		Phase:        string(pod.Status.Phase),
		Reason:       pod.Status.Reason,
		NodeName:     pod.Spec.NodeName,
		IP:           pod.Status.PodIP,
		Ready:        podReady(pod),
		RestartCount: restarts,
		Containers:   containers,
		Labels:       cloneStringMap(pod.Labels),
	}
}

func worldServiceFromKubernetes(cluster string, svc v1.Service) *WorldService {
	ports := make([]string, 0, len(svc.Spec.Ports))
	for _, port := range svc.Spec.Ports {
		name := port.Name
		if name == "" {
			name = fmt.Sprintf("%d", port.Port)
		}
		ports = append(ports, fmt.Sprintf("%s:%d/%s", name, port.Port, port.Protocol))
	}
	sort.Strings(ports)
	return &WorldService{
		ID:        worldID(cluster, svc.Namespace, "Service", svc.Name),
		Cluster:   cluster,
		Namespace: svc.Namespace,
		Name:      svc.Name,
		Type:      string(svc.Spec.Type),
		ClusterIP: svc.Spec.ClusterIP,
		Selector:  cloneStringMap(svc.Spec.Selector),
		Ports:     ports,
	}
}

func worldIngressFromKubernetes(cluster string, ingress networkingv1.Ingress) *WorldIngress {
	hosts := []string{}
	for _, rule := range ingress.Spec.Rules {
		if rule.Host != "" {
			hosts = appendUnique(hosts, rule.Host)
		}
	}
	return &WorldIngress{
		ID:        worldID(cluster, ingress.Namespace, "Ingress", ingress.Name),
		Cluster:   cluster,
		Namespace: ingress.Namespace,
		Name:      ingress.Name,
		Hosts:     hosts,
	}
}

func worldNodeFromKubernetes(node v1.Node) *WorldNode {
	vendor, region, instanceType := finops.ParseNodeMetadata(node.Labels, node.Spec.ProviderID)
	return &WorldNode{
		ID:           worldNodeID(node.Name),
		Name:         node.Name,
		ProviderID:   node.Spec.ProviderID,
		Vendor:       vendor,
		Region:       region,
		Zone:         firstNonEmpty(node.Labels["topology.kubernetes.io/zone"], node.Labels["failure-domain.beta.kubernetes.io/zone"]),
		InstanceType: instanceType,
		Ready:        nodeReady(node),
		Labels:       cloneStringMap(node.Labels),
	}
}

func applyDeploymentToWorldWorkload(w *WorldWorkload, deploy appsv1.Deployment) {
	desired := int32(1)
	if deploy.Spec.Replicas != nil {
		desired = *deploy.Spec.Replicas
	}
	w.Selector = labelSelectorString(deploy.Spec.Selector, w.Selector)
	w.Desired = desired
	w.Ready = deploy.Status.ReadyReplicas
	w.Available = deploy.Status.AvailableReplicas
	w.Updated = deploy.Status.UpdatedReplicas
	w.Status = fmt.Sprintf("desired=%d ready=%d available=%d updated=%d", desired, w.Ready, w.Available, w.Updated)
	w.Labels = cloneStringMap(deploy.Labels)
	w.Annotations = cloneStringMap(deploy.Annotations)
}

func applyStatefulSetToWorldWorkload(w *WorldWorkload, sts appsv1.StatefulSet) {
	desired := int32(1)
	if sts.Spec.Replicas != nil {
		desired = *sts.Spec.Replicas
	}
	w.Selector = labelSelectorString(sts.Spec.Selector, w.Selector)
	w.Desired = desired
	w.Ready = sts.Status.ReadyReplicas
	w.Available = sts.Status.AvailableReplicas
	w.Updated = sts.Status.UpdatedReplicas
	w.Status = fmt.Sprintf("desired=%d ready=%d updated=%d", desired, w.Ready, w.Updated)
	w.Labels = cloneStringMap(sts.Labels)
	w.Annotations = cloneStringMap(sts.Annotations)
}

func applyDaemonSetToWorldWorkload(w *WorldWorkload, ds appsv1.DaemonSet) {
	w.Selector = labelSelectorString(ds.Spec.Selector, w.Selector)
	w.Desired = ds.Status.DesiredNumberScheduled
	w.Ready = ds.Status.NumberReady
	w.Available = ds.Status.NumberAvailable
	w.Updated = ds.Status.UpdatedNumberScheduled
	w.Status = fmt.Sprintf("desired=%d ready=%d available=%d updated=%d", w.Desired, w.Ready, w.Available, w.Updated)
	w.Labels = cloneStringMap(ds.Labels)
	w.Annotations = cloneStringMap(ds.Annotations)
}

func applyJobToWorldWorkload(w *WorldWorkload, job batchv1.Job) {
	w.Selector = labelSelectorString(job.Spec.Selector, w.Selector)
	if job.Spec.Completions != nil {
		w.Desired = int32(*job.Spec.Completions)
	}
	w.Ready = job.Status.Succeeded
	w.Status = fmt.Sprintf("succeeded=%d failed=%d active=%d", job.Status.Succeeded, job.Status.Failed, job.Status.Active)
	w.Labels = cloneStringMap(job.Labels)
	w.Annotations = cloneStringMap(job.Annotations)
}

func worldID(cluster, namespace, kind, name string) string {
	namespace = firstNonEmpty(strings.TrimSpace(namespace), "_")
	return strings.Join([]string{strings.TrimSpace(cluster), namespace, strings.TrimSpace(kind), strings.TrimSpace(name)}, ":")
}

func worldNodeID(name string) string {
	return "node::" + strings.TrimSpace(name)
}

func podReady(pod v1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == v1.PodReady {
			return condition.Status == v1.ConditionTrue
		}
	}
	return false
}

func nodeReady(node v1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == v1.NodeReady {
			return condition.Status == v1.ConditionTrue
		}
	}
	return false
}

func ingressServiceNames(ingress networkingv1.Ingress) []string {
	seen := map[string]bool{}
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name != "" {
			seen[name] = true
		}
	}
	if ingress.Spec.DefaultBackend != nil && ingress.Spec.DefaultBackend.Service != nil {
		add(ingress.Spec.DefaultBackend.Service.Name)
	}
	for _, rule := range ingress.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}
		for _, path := range rule.HTTP.Paths {
			if path.Backend.Service != nil {
				add(path.Backend.Service.Name)
			}
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func appendUnique(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
