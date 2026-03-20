package model

import (
	"os"

	"github.com/muulinCorp/interlib/util"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

// BridgeConf 描述單一橋接規則，包含來源、目的端 MQTT 設定與裝置過濾清單
type BridgeConf struct {
	Dest    MqttEndpoint `yaml:"dest"`
	Source  MqttEndpoint `yaml:"source"`
	Devices []gwDevice   `yaml:"devices"`
}

// MqttEndpoint 描述一個 MQTT 端點的連線資訊與主題
type MqttEndpoint struct {
	Host      string `yaml:"host"`
	Port      string `yaml:"port"`
	Username  string `yaml:"username"`
	Password  string `yaml:"password"`
	DataTopic string `yaml:"data_topic"`
	CmdTopic  string `yaml:"cmd_topic"`
	Qos       int    `yaml:"qos"`
}

// gwDevice 描述允許通過過濾的裝置
type gwDevice struct {
	Id  string `yaml:"id"`
	Mac string `yaml:"mac"`
}

// BridgeRule 是解析後、可直接使用的單一橋接規則
type BridgeRule struct {
	Dest     MqttEndpoint
	Source   MqttEndpoint
	// macSet: mac -> device id，用於快速過濾
	MacSet   map[string]string
}

// IsAllowed 檢查 payload 中的 MAC 是否在允許清單內，
// 同時回傳對應的 device id
func (r *BridgeRule) IsAllowed(mac string) (string, bool) {
	id, ok := r.MacSet[mac]
	return id, ok
}

// LoadBridgeRules 從 yaml 設定檔讀取並回傳所有橋接規則
func LoadBridgeRules(cfgPath string) ([]BridgeRule, error) {
	if !util.FileExists(cfgPath) {
		return nil, errors.Errorf("config file not exist: %s", cfgPath)
	}
	fileBytes, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, errors.Wrap(err, "read config file fail")
	}

	var confs []BridgeConf
	if err = yaml.Unmarshal(fileBytes, &confs); err != nil {
		return nil, errors.Wrap(err, "unmarshal config file fail")
	}

	rules := make([]BridgeRule, 0, len(confs))
	for _, conf := range confs {
		macSet := make(map[string]string, len(conf.Devices))
		for _, d := range conf.Devices {
			macSet[d.Mac] = d.Id
		}
		rules = append(rules, BridgeRule{
			Dest:   conf.Dest,
			Source: conf.Source,
			MacSet: macSet,
		})
	}
	return rules, nil
}