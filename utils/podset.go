package utils

import (
	corev1 "k8s.io/api/core/v1"
	"sync"
)

type PodSet struct {
	set   map[string]*corev1.Pod
	mutex sync.Mutex
}

func NewPodSet() *PodSet {
	return &PodSet{
		set:   make(map[string]*corev1.Pod),
		mutex: sync.Mutex{},
	}
}

func (s *PodSet) Add(pod *corev1.Pod) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.set[podId(pod)] = pod
}

func (s *PodSet) Remove(pod *corev1.Pod) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	delete(s.set, podId(pod))
}

func (s *PodSet) HasGroup(pod *corev1.Pod) bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if podGroup := GetPodGroupName(pod); podGroup != nil {
		for _, pod := range s.set {
			if groupName := GetPodGroupName(pod); groupName != nil {
				if *podGroup == *groupName {
					return true
				}
			}
		}
	}
	return false
}

func (s *PodSet) Has(pod *corev1.Pod) bool {
	return s.HasId(podId(pod))
}

func (s *PodSet) HasId(pod string) bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	_, found := s.set[pod]
	return found
}

func podId(pod *corev1.Pod) string {
	return pod.Name
}
