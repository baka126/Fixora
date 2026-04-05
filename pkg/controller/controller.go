package controller

import (
	"context"
	"fmt"
	"time"

	"fixora/pkg/config"
	"fixora/pkg/notifications"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type Controller struct {
	clientset kubernetes.Interface
	factory   informers.SharedInformerFactory
	config    *config.Config
}

func NewController(clientset kubernetes.Interface, cfg *config.Config) *Controller {
	factory := informers.NewSharedInformerFactory(clientset, time.Second*30)
	return &Controller{
		clientset: clientset,
		factory:   factory,
		config:    cfg,
	}
}

func (c *Controller) Run(stopCh <-chan struct{}) {
	podInformer := c.factory.Core().V1().Pods().Informer()
	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*v1.Pod)
			fmt.Printf("Pod ADDED: %s/%s
", pod.Namespace, pod.Name)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.handleUpdate(oldObj, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			pod := obj.(*v1.Pod)
			fmt.Printf("Pod DELETED: %s/%s
", pod.Namespace, pod.Name)
		},
	})

	c.factory.Start(stopCh)

	if !cache.WaitForCacheSync(stopCh, podInformer.HasSynced) {
		return
	}

	<-stopCh
}

func (c *Controller) handleUpdate(oldObj, newObj interface{}) {
	pod := newObj.(*v1.Pod)
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Waiting != nil && containerStatus.State.Waiting.Reason == "CrashLoopBackOff" {
			c.handleCrashLoopBackOff(pod)
		}
		if containerStatus.State.Terminated != nil && containerStatus.State.Terminated.Reason == "OOMKilled" {
			c.handleOOMKilled(pod)
		}
	}
}

func (c *Controller) handleCrashLoopBackOff(pod *v1.Pod) {
	fmt.Printf("Pod %s/%s is in CrashLoopBackOff
", pod.Namespace, pod.Name)
	notifications.SendNotification(c.config, fmt.Sprintf("Pod %s/%s is in CrashLoopBackOff", pod.Namespace, pod.Name))

	owner := metav1.GetControllerOf(pod)
	if owner == nil {
		fmt.Printf("Pod %s/%s has no owner
", pod.Namespace, pod.Name)
		return
	}

	if owner.APIVersion != "apps/v1" || owner.Kind != "ReplicaSet" {
		return
	}

	rs, err := c.clientset.AppsV1().ReplicaSets(pod.Namespace).Get(context.TODO(), owner.Name, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Error getting ReplicaSet %s/%s: %s
", pod.Namespace, owner.Name, err.Error())
		return
	}

	rsOwner := metav1.GetControllerOf(rs)
	if rsOwner == nil {
		fmt.Printf("ReplicaSet %s/%s has no owner
", rs.Namespace, rs.Name)
		return
	}

	if rsOwner.APIVersion != "apps/v1" || rsOwner.Kind != "Deployment" {
		return
	}

	switch c.config.Mode {
	case config.DryRun:
		message := fmt.Sprintf("[Dry Run] Would have triggered rollout restart for Deployment %s/%s", rs.Namespace, rsOwner.Name)
		fmt.Println(message)
		notifications.SendNotification(c.config, message)
		return
	case config.AutoFix:
		c.PerformRolloutRestart(rs.Namespace, rsOwner.Name)
	case config.ClickToFix:
		c.requestApproval(rs.Namespace, rsOwner.Name)
	}
}

func (c *Controller) handleOOMKilled(pod *v1.Pod) {
	fmt.Printf("Pod %s/%s is OOMKilled
", pod.Namespace, pod.Name)
	notifications.SendNotification(c.config, fmt.Sprintf("Pod %s/%s is OOMKilled", pod.Namespace, pod.Name))
	// TODO: Implement OOMKilled remediation
}

func (c *Controller) PerformRolloutRestart(namespace, deploymentName string) {
	deployment, err := c.clientset.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Error getting Deployment %s/%s: %s
", namespace, deploymentName, err.Error())
		return
	}

	fmt.Printf("Triggering rollout restart for Deployment %s/%s
", deployment.Namespace, deployment.Name)
	notifications.SendNotification(c.config, fmt.Sprintf("Triggering rollout restart for Deployment %s/%s", deployment.Namespace, deployment.Name))

	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = make(map[string]string)
	}
	deployment.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

	_, err = c.clientset.AppsV1().Deployments(deployment.Namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
	if err != nil {
		fmt.Printf("Error updating Deployment %s/%s: %s
", deployment.Namespace, deployment.Name, err.Error())
		return
	}
}

func (c *Controller) requestApproval(namespace, deploymentName string) {
	callbackID := fmt.Sprintf("rollout-restart-%s-%s", namespace, deploymentName)
	message := fmt.Sprintf("Approval required to rollout restart Deployment %s/%s", namespace, deploymentName)
	err := notifications.SendInteractiveNotification(c.config, message, callbackID)
	if err != nil {
		fmt.Printf("Error sending interactive notification: %s
", err.Error())
	}
}
