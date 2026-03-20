package destination

import (
	"context"

	"github.com/94peter/mqtt"
	mqttCfg "github.com/94peter/mqtt/config"
	"github.com/pkg/errors"
)

// RawPublisher 將原始 bytes 直接發布到固定 topic，不做任何轉換
type RawPublisher interface {
	Publish(payload []byte) error
}

type rawPublisherImpl struct {
	topic    string
	qos      byte
	mqttServ mqtt.MqttServer
}

// NewRawPublisher 建立一個只負責 publish 的 MQTT client
func NewRawPublisher(ctx context.Context, cfg *mqttCfg.Config) (RawPublisher, error) {
	topic := cfg.Topics[0]

	serv, err := mqtt.NewMqttPublishOnlyServ(cfg)
	if err != nil {
		return nil, err
	}

	go serv.Run(ctx)

	return &rawPublisherImpl{
		topic:    topic,
		qos:      cfg.Qos,
		mqttServ: serv,
	}, nil
}

func (p *rawPublisherImpl) Publish(payload []byte) error {
	err := p.mqttServ.Publish(p.topic, p.qos, payload)
	if err != nil {
		return errors.Wrap(err, "mqtt publish fail")
	}
	return nil
}