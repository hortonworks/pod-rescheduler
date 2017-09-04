package main

import (
	"flag"
	"os"
	"path/filepath"
	"time"

	log "github.com/Sirupsen/logrus"
	utils "github.com/hortonworks/pod-rescheduler/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"text/tabwriter"
)

var (
	Version              string
	BuildTime            string
	housekeepingInterval = flag.Duration("housekeeping-interval", 10*time.Second, `How often rescheduler takes actions.`)
	namespace            = flag.String("namespace", metav1.NamespaceDefault, `Namespace to watch for Pods.`)
	minReplica           = flag.Int("min-replica-count", 2, "Minimum number or replicas that a replica set or replication controller should have to allow their pods deletion in scale down")
	podSchedulingTimeout = flag.Duration("pod-scheduled-timeout", 1*time.Minute, "How long should the rescheduler wait for a Pod to be scheduled")
)

func main() {
	formatter := &utils.TimeFormatter{}
	log.SetFormatter(formatter)

	log.Infof("Started pod-rescheduler application %s-%s", Version, BuildTime)

	config, err := rest.InClusterConfig()
	if err != nil {
		log.Warnf("Cannot use service account (/var/run/secrets/kubernetes.io/serviceaccount/" +
			corev1.ServiceAccountTokenKey + ") trying to connect with kube config file..")
	}

	if config == nil {
		var kubeconfig *string
		if home := homeDir(); home != "" {
			kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
		} else {
			kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
		}
		log.Infof("Use kube config: %s", *kubeconfig)
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			panic(err.Error())
		}
	}
	flag.Parse()

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	log.Info("Kubernetes client initialized")
	log.Info("Namespace: ", *namespace)
	log.Info("Housekeeping interval: ", housekeepingInterval)
	log.Info("Minimum replica count: ", *minReplica)
	log.Info("Pod scheduling timeout: ", podSchedulingTimeout)

	podClient := clientSet.CoreV1().Pods(*namespace)
	nodeClient := clientSet.Nodes()

	tabWriter := tabwriter.NewWriter(os.Stdout, 0, 0, 1, '.', tabwriter.Debug)
	log.SetOutput(tabWriter)

	for {
		select {
		case <-time.After(*housekeepingInterval):
			{
				nodes, err := nodeClient.List(metav1.ListOptions{})
				if err != nil {
					log.Errorf("Node list error: %s", err.Error())
					continue
				}

				var podsPerNode = make(map[string][]corev1.Pod)
				var allPods = make([]corev1.Pod, 0)
				for _, node := range nodes.Items {
					// ignore tainted nodes for now..
					if !node.Spec.Unschedulable && len(node.Spec.Taints) == 0 {
						pods := listPodsOnNode(podClient.List, node)
						podsPerNode[node.Name] = pods
						allPods = append(allPods, pods...)
					}
				}

				podGroups := groupPods(allPods)
				logPods(podGroups)
				for group, pods := range podGroups {
					if pod := findMovablePod(pods); pod != nil {
						log.Infof("Find node candidate for Pod: %s", pod.Name)
						if node := findNodeForPod(podsPerNode, group, nodes.Items); node != nil {
							// consider Taints and Tolerations to make sure it gets scheduled to the desired node
							log.Infof("Delete Pod (%s) in order to reschedule it to another node", pod.Name)
							err := podClient.Delete(pod.Name, &metav1.DeleteOptions{})
							if err != nil {
								log.Errorf("Failed to delete Pod: %s, error: %s", pod.Name, err.Error())
							}
						} else {
							log.Infof("There is no node candidate to move the Pod (%s) to", pod.Name)
						}
					} else {
						log.Infof("No action required for Pod group: %s", group)
					}
				}
			}
		}
	}
}

func logPods(podGroups map[string][]corev1.Pod) {
	for _, pods := range podGroups {
		for _, pod := range pods {
			log.Infof("%s\t%s\t%s\t%s", pod.Name, pod.Status.Phase, pod.Status.PodIP, pod.Spec.NodeName)
		}
	}
}

// Group the Pods that belong to the same Deployment/StatefulSet
// Single Pods are ignored
func groupPods(pods []corev1.Pod) (result map[string][]corev1.Pod) {
	result = make(map[string][]corev1.Pod)
	for _, pod := range pods {
		groupName := utils.GetPodGroupName(&pod)
		if groupName != nil {
			result[*groupName] = append(result[*groupName], pod)
		}
	}
	return result
}

// Find a Pod which has an alternative Running and Ready Pod on the same node
// Pods can be moved only when the minimum replica count is met
func findMovablePod(pods []corev1.Pod) *corev1.Pod {
	var podsByNode = make(map[string][]corev1.Pod)
	var podCandidate *corev1.Pod
	podCount := 0
	for i, pod := range pods {
		containerStatuses := pod.Status.ContainerStatuses
		if pod.Status.Phase == corev1.PodRunning && len(containerStatuses) > 0 {
			if isPodReady(&pod) {
				node := pod.Spec.NodeName
				if len(podsByNode[node]) == 1 {
					log.Infof("Pod: %s can be rescheduled as there is another running and ready pod (%s) on the same node: %s", pod.Name, podsByNode[node][0].Name, node)
					podCandidate = &pods[i]
				}
				podsByNode[node] = append(podsByNode[node], pod)
				podCount++
			}
		}
	}
	if podCount >= *minReplica {
		return podCandidate
	}
	return nil
}

func isPodReady(pod *corev1.Pod) bool {
	for _, cStatus := range pod.Status.ContainerStatuses {
		if !cStatus.Ready {
			log.Infof("Pod (%s) is running, but it's container (%s) is not ready", pod.Name, cStatus.Name)
			return false
		}
	}
	return true
}

// Find a node which does not run any Pod from the same Deployment/StatefulSet
func findNodeForPod(podsPerNode map[string][]corev1.Pod, group string, nodes []corev1.Node) *corev1.Node {
	for nodeName, pods := range podsPerNode {
		podFoundForGroup := false
		for _, pod := range pods {
			groupName := utils.GetPodGroupName(&pod)
			if groupName != nil && *groupName == group {
				log.Infof("Found Pod group(%s) on node: %s, searching..", group, nodeName)
				podFoundForGroup = true
				break
			}
		}
		if !podFoundForGroup {
			log.Infof("Found node: %s for Pod group: %s", nodeName, group)
			return findNode(nodeName, nodes)
		}
	}
	return nil
}

func findNode(name string, nodes []corev1.Node) *corev1.Node {
	for _, node := range nodes {
		if node.Name == name {
			return &node
		}
	}
	return nil
}

func listPodsOnNode(ListPodsOnNode func(opts metav1.ListOptions) (*corev1.PodList, error), node corev1.Node) []corev1.Pod {
	log.Infof("List Pods on node: %s", node.Name)
	podsOnNode, err := ListPodsOnNode(metav1.ListOptions{FieldSelector: fields.SelectorFromSet(fields.Set{"spec.nodeName": node.Name}).String()})
	if err != nil {
		log.Errorf("Failed to list Pods on node: %s", node.Name)
		return nil
	}
	return podsOnNode.Items
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

func waitForPodReadiness(getPod func(name string, options metav1.GetOptions) (*corev1.Pod, error), podsBeingProcessed *utils.PodSet, pod *corev1.Pod) {
	podName := pod.Name
	log.Infof("Waiting for pod %s to be scheduled", podName)
	err := wait.Poll(2*time.Second, *podSchedulingTimeout, func() (bool, error) {
		actualPod, err := getPod(pod.Name, metav1.GetOptions{})
		if err != nil {
			log.Warningf("Error while getting pod %s: %v", podName, err)
			return false, nil
		}
		return isPodReady(actualPod), nil
	})
	if err != nil {
		log.Warningf("Timeout while waiting for pod %s to be scheduled after %v.", podName, *podSchedulingTimeout)
	} else {
		log.Infof("Pod %v was successfully scheduled.", podName)
	}
	podsBeingProcessed.Remove(pod)
}
