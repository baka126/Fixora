package controller

import (
	"context"
	"fmt"
	"sort"
	"strings"

	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type ResourceCorrelation struct {
	lines   []string
	Related []string
	Scores  map[string]CorrelationScore
}

type CorrelationScore struct {
	Ref     string
	Score   int
	Reasons []string
}

type workloadIdentity struct {
	Kind     string
	Name     string
	Selector string
}

func (r ResourceCorrelation) Summary() string {
	if len(r.lines) == 0 {
		return "Resource Correlation: no related resource context found."
	}
	lines := append([]string{}, r.lines...)
	if top := r.TopCorrelations(5); len(top) > 0 {
		var parts []string
		for _, item := range top {
			parts = append(parts, fmt.Sprintf("%s=%d (%s)", item.Ref, item.Score, strings.Join(item.Reasons, "; ")))
		}
		lines = append(lines, "Top correlated resources: "+strings.Join(parts, " | "))
	}
	return "Resource Correlation:\n- " + strings.Join(lines, "\n- ")
}

func (r *ResourceCorrelation) add(line string) {
	line = strings.TrimSpace(line)
	if line != "" {
		r.lines = append(r.lines, line)
	}
}

func (r *ResourceCorrelation) relate(kind, name string) {
	if kind == "" || name == "" {
		return
	}
	r.score(kind, name, 25, "topology neighbor")
	r.Related = append(r.Related, kind+"/"+name)
}

func (r *ResourceCorrelation) score(kind, name string, score int, reason string) {
	if kind == "" || name == "" || score <= 0 {
		return
	}
	if r.Scores == nil {
		r.Scores = map[string]CorrelationScore{}
	}
	ref := kind + "/" + name
	current := r.Scores[ref]
	current.Ref = ref
	current.Score += score
	if current.Score > 100 {
		current.Score = 100
	}
	reason = strings.TrimSpace(reason)
	if reason != "" && !stringSliceContains(current.Reasons, reason) {
		current.Reasons = append(current.Reasons, reason)
	}
	r.Scores[ref] = current
}

func (r ResourceCorrelation) TopCorrelations(limit int) []CorrelationScore {
	if len(r.Scores) == 0 || limit <= 0 {
		return nil
	}
	out := make([]CorrelationScore, 0, len(r.Scores))
	for _, score := range r.Scores {
		out = append(out, score)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].Ref < out[j].Ref
		}
		return out[i].Score > out[j].Score
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (c *Controller) correlatePodResources(ctx context.Context, pod *v1.Pod) ResourceCorrelation {
	var corr ResourceCorrelation
	c.correlateOwners(ctx, pod, &corr)
	c.correlateServices(ctx, pod, &corr)
	c.correlateStorage(ctx, pod, &corr)
	c.correlateConfigRefs(ctx, pod, &corr)
	c.correlateNode(ctx, pod, &corr)
	c.correlateNetworkPolicies(ctx, pod, &corr)
	corr.Related = uniqueSorted(corr.Related)
	return corr
}

func (c *Controller) workloadIdentityForPod(ctx context.Context, pod *v1.Pod) workloadIdentity {
	fallback := workloadIdentity{
		Kind:     "Pod",
		Name:     pod.Name,
		Selector: labels.SelectorFromSet(labels.Set(pod.Labels)).String(),
	}

	rootKind, rootName := c.findTopLevelOwner(ctx, pod.Namespace, pod.OwnerReferences, pod.Labels)
	if rootKind == "" || rootName == "" {
		return fallback
	}

	// Try to fetch the selector for the root owner
	selector := fallback.Selector
	switch rootKind {
	case "Deployment":
		if obj, err := c.clientset.AppsV1().Deployments(pod.Namespace).Get(ctx, rootName, metav1.GetOptions{}); err == nil {
			selector = labelSelectorString(obj.Spec.Selector, selector)
		}
	case "StatefulSet":
		if obj, err := c.clientset.AppsV1().StatefulSets(pod.Namespace).Get(ctx, rootName, metav1.GetOptions{}); err == nil {
			selector = labelSelectorString(obj.Spec.Selector, selector)
		}
	case "DaemonSet":
		if obj, err := c.clientset.AppsV1().DaemonSets(pod.Namespace).Get(ctx, rootName, metav1.GetOptions{}); err == nil {
			selector = labelSelectorString(obj.Spec.Selector, selector)
		}
	case "Job":
		if obj, err := c.clientset.BatchV1().Jobs(pod.Namespace).Get(ctx, rootName, metav1.GetOptions{}); err == nil {
			selector = labelSelectorString(obj.Spec.Selector, selector)
		}
	case "CronJob":
		if obj, err := c.clientset.BatchV1().CronJobs(pod.Namespace).Get(ctx, rootName, metav1.GetOptions{}); err == nil {
			selector = labelSelectorString(obj.Spec.JobTemplate.Spec.Selector, selector)
		}
	}

	return workloadIdentity{
		Kind:     rootKind,
		Name:     rootName,
		Selector: selector,
	}
}

func (c *Controller) findTopLevelOwner(ctx context.Context, namespace string, owners []metav1.OwnerReference, podLabels map[string]string) (string, string) {
	for _, owner := range owners {
		parentKind, parentName := c.findTopLevelOwnerForResource(ctx, namespace, owner.Kind, owner.Name, podLabels)
		if parentKind != "" {
			return parentKind, parentName
		}
	}
	return "", ""
}

func (c *Controller) findTopLevelOwnerForResource(ctx context.Context, namespace, kind, name string, podLabels map[string]string) (string, string) {
	var parentOwners []metav1.OwnerReference

	switch kind {
	case "ReplicaSet":
		rs, err := c.clientset.AppsV1().ReplicaSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			// RS is missing but might be a standard deployment name pattern
			if podLabels != nil {
				if _, ok := podLabels["pod-template-hash"]; ok {
					if index := strings.LastIndex(name, "-"); index > 0 {
						deploymentName := name[:index]
						return c.findTopLevelOwnerForResource(ctx, namespace, "Deployment", deploymentName, podLabels)
					}
				}
			}
			return "ReplicaSet", name
		}
		parentOwners = rs.OwnerReferences
	case "Job":
		job, err := c.clientset.BatchV1().Jobs(namespace).Get(ctx, name, metav1.GetOptions{})
		if err == nil {
			parentOwners = job.OwnerReferences
		}
	case "Deployment", "StatefulSet", "DaemonSet", "CronJob":
		return kind, name
	default:
		// Unknown or custom controller, try to see if it has owners
		return kind, name
	}

	if len(parentOwners) > 0 {
		for _, parent := range parentOwners {
			pk, pn := c.findTopLevelOwnerForResource(ctx, namespace, parent.Kind, parent.Name, podLabels)
			if pk != "" {
				return pk, pn
			}
		}
	}

	return kind, name
}

func labelSelectorString(selector *metav1.LabelSelector, fallback string) string {
	if selector == nil {
		return fallback
	}
	parsed, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil || parsed.Empty() {
		return fallback
	}
	return parsed.String()
}

func (c *Controller) correlateOwners(ctx context.Context, pod *v1.Pod, corr *ResourceCorrelation) {
	chain := []string{"Pod/" + pod.Name}
	c.recursiveCorrelateOwners(ctx, pod.Namespace, pod.OwnerReferences, &chain, corr)
	corr.add("Owner chain: " + strings.Join(chain, " -> "))
}

func (c *Controller) recursiveCorrelateOwners(ctx context.Context, namespace string, owners []metav1.OwnerReference, chain *[]string, corr *ResourceCorrelation) {
	for _, owner := range owners {
		*chain = append(*chain, owner.Kind+"/"+owner.Name)
		corr.relate(owner.Kind, owner.Name)
		// Higher score for workload owners to ensure they outrank unrelated resources like ConfigMaps
		corr.score(owner.Kind, owner.Name, 50, "workload owner reference")

		var parentOwners []metav1.OwnerReference
		switch owner.Kind {
		case "ReplicaSet":
			rs, err := c.clientset.AppsV1().ReplicaSets(namespace).Get(ctx, owner.Name, metav1.GetOptions{})
			if err == nil {
				parentOwners = rs.OwnerReferences
			}
		case "Job":
			job, err := c.clientset.BatchV1().Jobs(namespace).Get(ctx, owner.Name, metav1.GetOptions{})
			if err == nil {
				parentOwners = job.OwnerReferences
			}
		case "Deployment":
			c.correlateDeployment(ctx, namespace, owner.Name, corr)
		case "StatefulSet":
			c.correlateStatefulSet(ctx, namespace, owner.Name, corr)
		case "DaemonSet":
			c.correlateDaemonSet(ctx, namespace, owner.Name, corr)
		}

		if len(parentOwners) > 0 {
			c.recursiveCorrelateOwners(ctx, namespace, parentOwners, chain, corr)
		}
	}
}

func (c *Controller) correlateDeployment(ctx context.Context, namespace, name string, corr *ResourceCorrelation) {
	deploy, err := c.clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return
	}
	desired := int32(1)
	if deploy.Spec.Replicas != nil {
		desired = *deploy.Spec.Replicas
	}
	corr.add(fmt.Sprintf("Deployment %s rollout: desired=%d updated=%d available=%d unavailable=%d", name, desired, deploy.Status.UpdatedReplicas, deploy.Status.AvailableReplicas, deploy.Status.UnavailableReplicas))
	if deploy.Status.UnavailableReplicas > 0 || deploy.Status.AvailableReplicas < desired {
		corr.score("Deployment", name, 30, "rollout availability mismatch")
	}
	for _, cond := range deploy.Status.Conditions {
		if cond.Status != v1.ConditionTrue {
			corr.add(fmt.Sprintf("Deployment %s condition %s=%s reason=%s", name, cond.Type, cond.Status, cond.Reason))
			corr.score("Deployment", name, 20, "deployment condition "+string(cond.Type)+"="+string(cond.Status))
		}
	}
}

func (c *Controller) correlateStatefulSet(ctx context.Context, namespace, name string, corr *ResourceCorrelation) {
	sts, err := c.clientset.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return
	}
	corr.add(fmt.Sprintf("StatefulSet %s rollout: replicas=%d ready=%d updated=%d", name, sts.Status.Replicas, sts.Status.ReadyReplicas, sts.Status.UpdatedReplicas))
	if sts.Status.ReadyReplicas < sts.Status.Replicas {
		corr.score("StatefulSet", name, 30, "statefulset ready replicas below desired")
	}
}

func (c *Controller) correlateDaemonSet(ctx context.Context, namespace, name string, corr *ResourceCorrelation) {
	ds, err := c.clientset.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return
	}
	corr.add(fmt.Sprintf("DaemonSet %s rollout: desired=%d ready=%d unavailable=%d", name, ds.Status.DesiredNumberScheduled, ds.Status.NumberReady, ds.Status.NumberUnavailable))
	if ds.Status.NumberUnavailable > 0 || ds.Status.NumberReady < ds.Status.DesiredNumberScheduled {
		corr.score("DaemonSet", name, 30, "daemonset ready count below desired")
	}
}

func (c *Controller) correlateServices(ctx context.Context, pod *v1.Pod, corr *ResourceCorrelation) {
	services, err := c.clientset.CoreV1().Services(pod.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return
	}
	for _, svc := range services.Items {
		if !selectorMatches(svc.Spec.Selector, pod.Labels) {
			continue
		}
		corr.relate("Service", svc.Name)
		endpoints, err := c.clientset.CoreV1().Endpoints(pod.Namespace).Get(ctx, svc.Name, metav1.GetOptions{})
		if err != nil {
			corr.add(fmt.Sprintf("Service %s selects pod labels but endpoints are missing: %v", svc.Name, err))
			corr.score("Service", svc.Name, 40, "service endpoints missing")
			continue
		}
		if endpointsIncludePod(endpoints, pod.Name) {
			corr.add(fmt.Sprintf("Service %s selects this pod and has an endpoint for it", svc.Name))
			corr.score("Service", svc.Name, 20, "service selector matches pod")
		} else {
			corr.add(fmt.Sprintf("Service %s selects this pod but endpoints do not include it", svc.Name))
			corr.score("Service", svc.Name, 55, "service endpoint excludes failing pod")
		}
		c.correlateIngressesForService(ctx, pod.Namespace, svc.Name, corr)
	}
}

func (c *Controller) correlateIngressesForService(ctx context.Context, namespace, serviceName string, corr *ResourceCorrelation) {
	ingresses, err := c.clientset.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return
	}
	for _, ingress := range ingresses.Items {
		if !ingressReferencesService(ingress, serviceName) {
			continue
		}
		corr.relate("Ingress", ingress.Name)
		corr.score("Ingress", ingress.Name, 20, "ingress routes to selected service")
		corr.add(fmt.Sprintf("Ingress %s routes traffic to Service %s", ingress.Name, serviceName))
	}
}

func ingressReferencesService(ingress networkingv1.Ingress, serviceName string) bool {
	if ingress.Spec.DefaultBackend != nil &&
		ingress.Spec.DefaultBackend.Service != nil &&
		ingress.Spec.DefaultBackend.Service.Name == serviceName {
		return true
	}
	for _, rule := range ingress.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}
		for _, path := range rule.HTTP.Paths {
			if path.Backend.Service != nil && path.Backend.Service.Name == serviceName {
				return true
			}
		}
	}
	return false
}

func (c *Controller) correlateStorage(ctx context.Context, pod *v1.Pod, corr *ResourceCorrelation) {
	for _, vol := range pod.Spec.Volumes {
		if vol.PersistentVolumeClaim == nil {
			continue
		}
		name := vol.PersistentVolumeClaim.ClaimName
		corr.relate("PersistentVolumeClaim", name)
		pvc, err := c.clientset.CoreV1().PersistentVolumeClaims(pod.Namespace).Get(ctx, name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			corr.add(fmt.Sprintf("PVC %s is referenced but missing", name))
			corr.score("PersistentVolumeClaim", name, 65, "referenced PVC is missing")
			continue
		}
		if err == nil {
			corr.add(fmt.Sprintf("PVC %s phase=%s volume=%s", name, pvc.Status.Phase, pvc.Spec.VolumeName))
			if pvc.Status.Phase != v1.ClaimBound {
				corr.score("PersistentVolumeClaim", name, 55, "PVC is not bound")
			}
		}
	}
}

func (c *Controller) correlateConfigRefs(ctx context.Context, pod *v1.Pod, corr *ResourceCorrelation) {
	for _, ref := range configRefsForPod(pod) {
		corr.relate(ref.Kind, ref.Name)
		switch ref.Kind {
		case "Secret":
			secret, err := c.clientset.CoreV1().Secrets(pod.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				corr.add(fmt.Sprintf("Secret %s is referenced but missing", ref.Name))
				corr.score("Secret", ref.Name, 70, "referenced Secret is missing")
				continue
			}
			if err == nil && ref.Key != "" {
				if _, ok := secret.Data[ref.Key]; !ok {
					corr.add(fmt.Sprintf("Secret %s is missing referenced key %s", ref.Name, ref.Key))
					corr.score("Secret", ref.Name, 70, "referenced Secret key is missing")
				}
			}
		case "ConfigMap":
			cm, err := c.clientset.CoreV1().ConfigMaps(pod.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				corr.add(fmt.Sprintf("ConfigMap %s is referenced but missing", ref.Name))
				corr.score("ConfigMap", ref.Name, 70, "referenced ConfigMap is missing")
				continue
			}
			if err == nil && ref.Key != "" {
				if _, ok := cm.Data[ref.Key]; !ok {
					corr.add(fmt.Sprintf("ConfigMap %s is missing referenced key %s", ref.Name, ref.Key))
					corr.score("ConfigMap", ref.Name, 70, "referenced ConfigMap key is missing")
				}
			}
		}
	}
}

func (c *Controller) correlateNode(ctx context.Context, pod *v1.Pod, corr *ResourceCorrelation) {
	if pod.Spec.NodeName == "" {
		return
	}
	node, err := c.clientset.CoreV1().Nodes().Get(ctx, pod.Spec.NodeName, metav1.GetOptions{})
	if err != nil {
		return
	}
	corr.relate("Node", node.Name)
	for _, cond := range node.Status.Conditions {
		switch cond.Type {
		case v1.NodeReady:
			if cond.Status != v1.ConditionTrue {
				corr.add(fmt.Sprintf("Node %s Ready=%s reason=%s", node.Name, cond.Status, cond.Reason))
				corr.score("Node", node.Name, 60, "node not ready")
			}
		case v1.NodeMemoryPressure, v1.NodeDiskPressure, v1.NodePIDPressure, v1.NodeNetworkUnavailable:
			if cond.Status == v1.ConditionTrue {
				corr.add(fmt.Sprintf("Node %s has %s reason=%s", node.Name, cond.Type, cond.Reason))
				corr.score("Node", node.Name, 55, "node condition "+string(cond.Type))
			}
		}
	}
}

func (c *Controller) correlateNetworkPolicies(ctx context.Context, pod *v1.Pod, corr *ResourceCorrelation) {
	policies, err := c.clientset.NetworkingV1().NetworkPolicies(pod.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return
	}
	var selected []string
	for _, policy := range policies.Items {
		if networkPolicySelectsPod(policy, pod.Labels) {
			selected = append(selected, policy.Name)
			corr.relate("NetworkPolicy", policy.Name)
			corr.score("NetworkPolicy", policy.Name, 15, "network policy selects pod")
		}
	}
	sort.Strings(selected)
	if len(selected) > 0 {
		corr.add("NetworkPolicies selecting this pod: " + strings.Join(selected, ", "))
	}
}

type configRef struct {
	Kind string
	Name string
	Key  string
}

func configRefsForPod(pod *v1.Pod) []configRef {
	seen := map[configRef]bool{}
	containers := append([]v1.Container{}, pod.Spec.InitContainers...)
	containers = append(containers, pod.Spec.Containers...)
	for _, container := range containers {
		for _, envFrom := range container.EnvFrom {
			if envFrom.SecretRef != nil {
				seen[configRef{Kind: "Secret", Name: envFrom.SecretRef.Name}] = true
			}
			if envFrom.ConfigMapRef != nil {
				seen[configRef{Kind: "ConfigMap", Name: envFrom.ConfigMapRef.Name}] = true
			}
		}
		for _, env := range container.Env {
			if env.ValueFrom == nil {
				continue
			}
			if env.ValueFrom.SecretKeyRef != nil {
				seen[configRef{Kind: "Secret", Name: env.ValueFrom.SecretKeyRef.Name, Key: env.ValueFrom.SecretKeyRef.Key}] = true
			}
			if env.ValueFrom.ConfigMapKeyRef != nil {
				seen[configRef{Kind: "ConfigMap", Name: env.ValueFrom.ConfigMapKeyRef.Name, Key: env.ValueFrom.ConfigMapKeyRef.Key}] = true
			}
		}
	}
	for _, vol := range pod.Spec.Volumes {
		if vol.Secret != nil {
			seen[configRef{Kind: "Secret", Name: vol.Secret.SecretName}] = true
		}
		if vol.ConfigMap != nil {
			seen[configRef{Kind: "ConfigMap", Name: vol.ConfigMap.Name}] = true
			for _, item := range vol.ConfigMap.Items {
				seen[configRef{Kind: "ConfigMap", Name: vol.ConfigMap.Name, Key: item.Key}] = true
			}
		}
	}

	refs := make([]configRef, 0, len(seen))
	for ref := range seen {
		if ref.Name != "" {
			refs = append(refs, ref)
		}
	}
	sort.Slice(refs, func(i, j int) bool {
		return refs[i].Kind+"/"+refs[i].Name+"/"+refs[i].Key < refs[j].Kind+"/"+refs[j].Name+"/"+refs[j].Key
	})
	return refs
}

func networkPolicySelectsPod(policy networkingv1.NetworkPolicy, podLabels map[string]string) bool {
	selector, err := metav1.LabelSelectorAsSelector(&policy.Spec.PodSelector)
	if err != nil {
		return false
	}
	return selector.Matches(labels.Set(podLabels))
}

func uniqueSorted(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		if value != "" {
			seen[value] = true
		}
	}
	return sortedKeys(seen)
}

func stringSliceContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
