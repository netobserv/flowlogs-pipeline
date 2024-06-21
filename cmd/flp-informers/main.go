package main

import (
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

var (
	buildVersion = "unknown"
	buildDate    = "unknown"
	app          = "flp-informers"
	configPath   = flag.String("config", "", "path to a config file")
	versionFlag  = flag.Bool("v", false, "print version")
	log          = logrus.WithField("module", "main")
)

func main() {
	flag.Parse()

	appVersion := fmt.Sprintf("%s [build version: %s, build date: %s]", app, buildVersion, buildDate)
	if *versionFlag {
		fmt.Println(appVersion)
		os.Exit(0)
	}

	cfg, err := readConfig(*configPath)
	if err != nil {
		log.WithError(err).Fatal("error reading config file")
	}

	lvl, err := logrus.ParseLevel(cfg.LogLevel)
	if err != nil {
		log.Errorf("Log level %s not recognized, using info", cfg.LogLevel)
		lvl = logrus.InfoLevel
	}
	logrus.SetLevel(lvl)
	log.Infof("Starting %s at log level %s", appVersion, lvl)
	log.Infof("Configuration: %#v", cfg)

	if cfg.PProfPort != 0 {
		go func() {
			log.WithField("port", cfg.PProfPort).Info("starting PProf HTTP listener")
			err := http.ListenAndServe(fmt.Sprintf(":%d", cfg.PProfPort), nil)
			log.WithError(err).Error("PProf HTTP listener stopped working")
		}()
	}

	opMetrics := operational.NewMetrics(&cfg.MetricsSettings)
	err = kubernetes.InitInformerDatasource(cfg.KubeConfig, &cfg.KafkaConfig, opMetrics)
	if err != nil {
		log.WithError(err).Fatal("error initializing Kubernetes & informers")
	}

	stopCh := utils.SetupElegantExit()
	<-stopCh
}

func readConfig(path string) (*config.Informers, error) {
	var cfg config.Informers
	if len(path) == 0 {
		return &cfg, nil
	}
	yamlFile, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(yamlFile, &cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, err
}
