package utils

import (
	"sync"

	corev1 "k8s.io/api/core/v1"
)

type PodSet struct {
	set   map[string]struct{}
	mutex sync.Mutex
}

func NewPodSet() *PodSet {
	return &PodSet{
		set:   make(map[string]struct{}),
		mutex: sync.Mutex{},
	}
}

func (s *PodSet) Add(pod *corev1.Pod) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.set[podId(pod)] = struct{}{}
}

func (s *PodSet) Remove(pod *corev1.Pod) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	delete(s.set, podId(pod))
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
