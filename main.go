package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
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
	for {

		// nodes
		nodes, err := clientSet.Nodes().List(v1.ListOptions{})
		if err != nil {
			fmt.Println("Node list error: ", err.Error())
		}
		fmt.Println("The following nodes are schedulable:")
		for _, node := range nodes.Items {
			if !node.Spec.Unschedulable {
				fmt.Println("Name:", node.GetName())
			}
		}

		// deployments
		deploymentClient := clientSet.AppsV1beta1().Deployments(v1.NamespaceDefault)

		deployments, err := deploymentClient.List(v1.ListOptions{})
		if err != nil {
			fmt.Println("Deployment list error: ", err.Error())
		}
		fmt.Println("Deployment list:")
		for _, deployment := range deployments.Items {
			fmt.Println("Name:", deployment.GetName())
		}

		// replicasets
		replicaSetClient := clientSet.ExtensionsV1beta1Client.ReplicaSets(v1.NamespaceDefault)

		rsList, err := replicaSetClient.List(v1.ListOptions{})
		if err != nil {
			fmt.Println("ReplicateSet list error: ", err.Error())
		}
		fmt.Println("ReplicateSet list:")
		for _, rs := range rsList.Items {
			fmt.Println("Name:", rs.Name)
		}

		// pods
		podClient := clientSet.CoreV1().Pods(v1.NamespaceDefault)

		pods, err := podClient.List(v1.ListOptions{})
		if err != nil {
			panic(err.Error())
		}
		fmt.Printf("There are %d pods in the cluster\n", len(pods.Items))


		_, err = clientSet.CoreV1().Pods(v1.NamespaceDefault).Get("cloudbreak-1690346111-5h5g9", v1.GetOptions{})
		if errors.IsNotFound(err) {
			fmt.Printf("Pod not found\n")
		} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
			fmt.Printf("Error getting pod %v\n", statusError.ErrStatus.Message)
		} else if err != nil {
			panic(err.Error())
		} else {
			fmt.Printf("Found pod\n")
		}

		time.Sleep(10 * time.Second)
	}
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}
