package netobserv

import (
	"github.com/stretchr/testify/mock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InformersMock provides an informers' implementation for unit testing
type InformersMock struct {
	mock.Mock
	InformersInterface
}

func (o *InformersMock) PodByIP(ip string) *corev1.Pod {
	args := o.Called(ip)
	return args.Get(0).(*corev1.Pod)
}

func (o *InformersMock) ServiceByIP(ip string) *corev1.Service {
	args := o.Called(ip)
	return args.Get(0).(*corev1.Service)
}

func (o *InformersMock) ReplicaSet(namespace, name string) *appsv1.ReplicaSet {
	args := o.Called(namespace, name)
	return args.Get(0).(*appsv1.ReplicaSet)
}

func fakePod(name, ns, host string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Status: corev1.PodStatus{
			HostIP: host,
		},
	}
}

func fakeService(name, ns string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
	}
}

func (o *InformersMock) MockPod(name, ns, ip, host string) {
	o.On("PodByIP", ip).Return(fakePod(name, ns, host))
}

func (o *InformersMock) MockPodInDepl(name, ns, ip, host, rs, depl string) {
	pod := fakePod(name, ns, host)
	pod.OwnerReferences = append(pod.OwnerReferences, metav1.OwnerReference{
		Name: rs,
		Kind: "ReplicaSet",
	})
	o.On("PodByIP", ip).Return(pod)
	o.On("ReplicaSet", ns, rs).Return(&appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rs,
			Namespace: ns,
			OwnerReferences: []metav1.OwnerReference{
				{
					Name: depl,
					Kind: "Deployment",
				},
			},
		},
	})
}

func (o *InformersMock) MockService(name, ns, ip string) {
	o.On("PodByIP", ip).Return((*corev1.Pod)(nil))
	o.On("ServiceByIP", ip).Return(fakeService(name, ns))
}

func (o *InformersMock) MockNoMatch(ip string) {
	o.On("PodByIP", ip).Return((*corev1.Pod)(nil))
	o.On("ServiceByIP", ip).Return((*corev1.Service)(nil))
}
