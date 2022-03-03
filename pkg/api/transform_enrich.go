package api

type KubeEnrich struct {
	KubeConfigPath string            `yaml:"kubeConfigPath" doc:"path to kubeconfig file (optional)"`
	IPFields       map[string]string `yaml:"ipFields" doc:"names of flow fields that contain an IP address with the prefix to prepend to the original field name (can be empty)"`
}
