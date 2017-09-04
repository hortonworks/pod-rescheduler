package utils

import corev1 "k8s.io/api/core/v1"

func GetPodGroupName(pod *corev1.Pod) *string {
	return getPodGroupName(pod.GenerateName)
}

func getPodGroupName(generateName string) *string {
	if len(generateName) > 0 {
		generateName = generateName[0 : len(generateName)-1]
		return &generateName
	}
	return nil
}
