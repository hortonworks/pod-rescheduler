package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	var kubeconfig *string
	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	podClient := clientSet.CoreV1().Pods(metav1.NamespaceDefault)
	nodeClient := clientSet.Nodes()

	for {
		// nodes
		nodes, err := nodeClient.List(metav1.ListOptions{})
		if err != nil {
			fmt.Println("Node list error: ", err.Error())
			continue
		}

		var podsPerNode = make(map[string][]corev1.Pod)
		var allPods = make([]corev1.Pod, 0)
		for _, node := range nodes.Items {
			if !node.Spec.Unschedulable {
				pods := listPodsOnNode(podClient.List, node)
				podsPerNode[node.Name] = pods
				allPods = append(allPods, pods...)
			}
		}

		podGroups := groupPods(allPods)
		for group, pods := range podGroups {
			if movablePod := findMovablePod(pods); movablePod != nil {
				if node := findNodeForPod(podsPerNode, group, nodes.Items); node != nil {
					fmt.Printf("Attempting to move Pod (%s) to node %s\n", movablePod.Name, node.Name)


				}
			}
		}

		time.Sleep(10 * time.Second)
	}
}

// Group the Pods that belong to the same Deployment/StatefulSet
func groupPods(pods []corev1.Pod) (result map[string][]corev1.Pod) {
	result = make(map[string][]corev1.Pod)
	for _, pod := range pods {
		groupName := getPodGroupName(pod)
		result[groupName] = append(result[groupName], pod)
	}
	return result
}

func getPodGroupName(pod corev1.Pod) string {
	return pod.GenerateName[0 : len(pod.GenerateName)-1]
}

// Find a Pod which has an alternative Running Pod on the same node
func findMovablePod(pods []corev1.Pod) *corev1.Pod {
	var podsByNode = make(map[string][]corev1.Pod)
	for _, pod := range pods {
		if pod.Status.Phase == corev1.PodRunning {
			node := pod.Spec.NodeName
			if len(podsByNode[node]) == 1 {
				fmt.Printf("Pod: %s can be moved as there is another running pod (%s) on the same node: %s\n", pod.Name, podsByNode[node][0].Name, node)
				return &pod
			}
			podsByNode[node] = append(podsByNode[node], pod)
		}
	}
	return nil
}

// Find a node which does not run any Pod from one Deployment/StatefulSet
func findNodeForPod(podsPerNode map[string][]corev1.Pod, group string, nodes []corev1.Node) *corev1.Node {
	fmt.Println("Find node for Pod group:", group)
	for node, pods := range podsPerNode {
		podFoundForGroup := false
		for _, pod := range pods {
			if getPodGroupName(pod) == group {
				fmt.Printf("Found Pod group(%s) on node: %s, searching..\n", group, node)
				podFoundForGroup = true
				break
			}
		}
		if !podFoundForGroup {
			fmt.Printf("Foud node: %s for Pod group: %s\n", node, group)
			return findNode(node, nodes)
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
	fmt.Println("List Pods on node:", node.Name)
	podsOnNode, err := ListPodsOnNode(metav1.ListOptions{FieldSelector: fields.SelectorFromSet(fields.Set{"spec.nodeName": node.Name}).String()})
	if err != nil {
		fmt.Println("Failed to list Pods on node:", node.Name)
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
