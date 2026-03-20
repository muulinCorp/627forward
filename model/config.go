package model

import (
	"github.com/94peter/log"
	"github.com/94peter/microservice/cfg"
	"github.com/94peter/microservice/di"
	"github.com/pkg/errors"
)

type ModelDI interface {
	di.DI
	log.LoggerDI
}

type Config struct {
	ForwardConfigFilePath string `env:"FORWARD_CONF_FILE"`

	di  ModelDI
	log log.Logger
}

func (c *Config) NewLog(uuid string) (log.Logger, error) {
	return c.di.NewLogger(c.di.GetService(), uuid)
}

func (c *Config) Close() error {
	return nil
}

func (c *Config) Init(uuid string, di di.DI) error {
	mdi, ok := di.(ModelDI)
	if !ok {
		return errors.New("no ModelDI when Config Init()")
	}

	c.di = mdi
	var err error
	c.log, err = c.NewLog(uuid)
	if err != nil {
		return err
	}

	return nil
}

func (c *Config) Copy() cfg.ModelCfg {
	cp := *c
	return &cp
}

func GetConfigFromEnv() (*Config, error) {
	var myCfg Config
	err := cfg.GetFromEnv(&myCfg)
	if err != nil {
		return nil, err
	}
	return &myCfg, nil
}