package model

import (
	"crypto/md5"
	"fmt"
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
	Dest   MqttEndpoint
	Source MqttEndpoint
	// MacSet: mac -> device id，用於 isAllowedPayload 過濾
	MacSet map[string]string
	// Md5TopicSet: md5hex -> mac，用於 dest cmd 動態訂閱與反查
	// key = md5(mac+id)，value = mac
	Md5TopicSet map[string]string
}

// DeviceCmdTopics 回傳此規則下所有 device 對應的 dest cmd 動態 topic 列表，
// 格式為 "<baseCmdTopic>/<md5hex>"
func (r *BridgeRule) DeviceCmdTopics(baseCmdTopic string) []string {
	topics := make([]string, 0, len(r.Md5TopicSet))
	for md5hex := range r.Md5TopicSet {
		topics = append(topics, fmt.Sprintf("%s/%s", baseCmdTopic, md5hex))
	}
	return topics
}

// CalcMd5Topic 計算單一 mac+gwID 組合的動態 topic
func CalcMd5Topic(baseCmdTopic, mac, gwID string) string {
	raw := fmt.Sprintf("%s%s", mac, gwID)
	hash := md5.Sum([]byte(raw))
	return fmt.Sprintf("%s/%x", baseCmdTopic, hash)
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
		md5TopicSet := make(map[string]string, len(conf.Devices))
		for _, d := range conf.Devices {
			macSet[d.Mac] = d.Id
			raw := fmt.Sprintf("%s%s", d.Mac, d.Id)
			hash := md5.Sum([]byte(raw))
			md5hex := fmt.Sprintf("%x", hash)
			md5TopicSet[md5hex] = d.Mac
		}
		rules = append(rules, BridgeRule{
			Dest:        conf.Dest,
			Source:      conf.Source,
			MacSet:      macSet,
			Md5TopicSet: md5TopicSet,
		})
	}
	return rules, nil
}