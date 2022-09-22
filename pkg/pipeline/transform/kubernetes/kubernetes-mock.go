package kubernetes

import "github.com/stretchr/testify/mock"

type KubeDataMock struct {
	mock.Mock
	kubeDataInterface
}

func (o *KubeDataMock) InitFromConfig(kubeConfigPath string) error {
	args := o.Called(kubeConfigPath)
	return args.Error(0)
}
