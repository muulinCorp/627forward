package main

import (
	"context"
	"mqtt-adaptor/model"
	"os"

	"github.com/94peter/log"
	"github.com/94peter/microservice"
	"github.com/94peter/microservice/di"
	"github.com/joho/godotenv"
	"github.com/muulinCorp/interlib/util"
	"github.com/pkg/errors"
)

func main() {
	path, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	envFile := path + "/.env"
	if util.FileExists(envFile) {
		err := godotenv.Load(envFile)
		if err != nil {
			panic(errors.Wrap(err, "load .env file fail"))
		}
	}

	modelCfg, err := model.GetConfigFromEnv()
	if err != nil {
		panic(err)
	}

	ms, err := microservice.New(modelCfg, &myDI{})
	if err != nil {
		panic(err)
	}

	serv := newService(ms)
	microservice.RunService(serv.startBridge)
}

type myDI struct {
	di.CommonServiceDI
	*log.LoggerConf `yaml:"log,omitempty"`
}

func (di *myDI) IsConfEmpty() error {
	if di.LoggerConf == nil {
		return errors.New("log config is empty")
	}
	if log.EnvHasFluentd() && (di.LoggerConf == nil || di.LoggerConf.FluentLog == nil) {
		return errors.New("fluentd config is empty")
	}
	return nil
}

type service struct {
	microservice.MicroService[*model.Config, *myDI]
}

func newService(ms microservice.MicroService[*model.Config, *myDI]) *service {
	return &service{MicroService: ms}
}

func (s *service) startBridge(ctx context.Context) {
	cfg, err := s.NewCfg("bridge")
	if err != nil {
		panic(err)
	}

	l, err := cfg.NewLog("MQTT Bridge")
	if err != nil {
		panic(err)
	}

	rules, err := model.LoadBridgeRules(cfg.ForwardConfigFilePath)
	if err != nil {
		panic(err)
	}

	if len(rules) == 0 {
		panic(errors.New("no bridge rules found in config"))
	}

	l.Infof("loaded %d bridge rule(s)", len(rules))

	bridges := make([]model.Bridge, 0, len(rules))
	for i, rule := range rules {
		b, err := model.NewBridge(ctx, rule, i)
		if err != nil {
			panic(errors.Wrapf(err, "create bridge[%d] fail", i))
		}
		bridges = append(bridges, b)
	}

	// 最後一個 bridge 在目前 goroutine 中 blocking 執行，其餘在背景執行
	for i := 0; i < len(bridges)-1; i++ {
		go bridges[i].Run(ctx)
	}
	bridges[len(bridges)-1].Run(ctx)
}