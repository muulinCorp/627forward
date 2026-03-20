package model

import (
	"context"
	"encoding/json"
	goLog "log"
	"net/url"
	"os"
	"strconv"

	dest "mqtt-adaptor/model/destination"

	"github.com/94peter/mqtt"
	mqttCfg "github.com/94peter/mqtt/config"
	"github.com/94peter/mqtt/trans"
	"github.com/pkg/errors"
)

// Bridge 管理單一橋接規則的雙向轉發
type Bridge interface {
	Run(ctx context.Context)
}

type bridgeImpl struct {
	// source sub → dest pub（data 方向）
	sourceSubServ mqtt.MqttSubOnlyServer
	// dest sub → source pub（cmd 方向）
	destSubServ mqtt.MqttSubOnlyServer
}

// makeMqttCfg 根據 MqttEndpoint 建立 *mqttCfg.Config，topic 由呼叫方指定
func makeMqttCfg(ep MqttEndpoint, topics []string, clientID string) (*mqttCfg.Config, error) {
	rawURL := ep.Host + ":" + ep.Port
	serverURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, errors.Wrapf(err, "parse url fail: %s", rawURL)
	}

	cfg := &mqttCfg.Config{
		ServerURL:         serverURL,
		ClientID:          clientID,
		Topics:            topics,
		Qos:               byte(ep.Qos),
		KeepAlive:         30,
		ConnectRetryDelay: 500,
		Debug:             true,
		Logger:            goLog.New(os.Stdout, "["+clientID+"] ", 0),
	}
	if ep.Username != "" && ep.Password != "" {
		cfg.SetAuth(ep.Username, []byte(ep.Password))
	}
	return cfg, nil
}

// NewBridge 根據一條 BridgeRule 建立雙向橋接：
//
//	source.data_topic  →  dest.data_topic   （rawdata 方向）
//	dest.cmd_topic     →  source.cmd_topic  （命令反向）
func NewBridge(ctx context.Context, rule BridgeRule, index int) (Bridge, error) {
	idxStr := strconv.Itoa(index)

	// ── 1. dest publisher（接收 source data，推送到 dest data_topic）
	destDataCfg, err := makeMqttCfg(rule.Dest, []string{rule.Dest.DataTopic}, "dest-pub-"+idxStr)
	if err != nil {
		return nil, err
	}
	destPub, err := dest.NewRawPublisher(ctx, destDataCfg)
	if err != nil {
		return nil, errors.Wrap(err, "create dest publisher fail")
	}

	// ── 2. source publisher（接收 dest cmd，推送到 source cmd_topic）
	srcCmdCfg, err := makeMqttCfg(rule.Source, []string{rule.Source.CmdTopic}, "src-pub-cmd-"+idxStr)
	if err != nil {
		return nil, err
	}
	srcCmdPub, err := dest.NewRawPublisher(ctx, srcCmdCfg)
	if err != nil {
		return nil, errors.Wrap(err, "create source cmd publisher fail")
	}

	// ── 3. source subscriber（訂閱 source data_topic）
	//      過濾裝置後，raw payload → destPub
	srcDataTrans := trans.NewSimpleTrans(func(topic string, payload []byte) error {
		if !isAllowedPayload(payload, rule) {
			return nil
		}
		return destPub.Publish(payload)
	})

	srcSubCfg, err := makeMqttCfg(rule.Source, []string{rule.Source.DataTopic}, "src-sub-"+idxStr)
	if err != nil {
		return nil, err
	}
	sourceSubServ, err := mqtt.NewMqttSubOnlyServ(srcSubCfg, map[string]trans.Trans{
		rule.Source.DataTopic: srcDataTrans,
	})
	if err != nil {
		return nil, errors.Wrap(err, "create source subscriber fail")
	}

	// ── 4. dest subscriber（訂閱每個 device 對應的動態 cmd topic）
	//      topic 格式：dest.cmd_topic/<md5(mac+gwID)>
	//      收到後直接轉發到 source 的對應動態 cmd topic
	destCmdTopics := rule.DeviceCmdTopics(rule.Dest.CmdTopic)
	var destSubServLocal mqtt.MqttSubOnlyServer
	if len(destCmdTopics) > 0 {
		destCmdTransMap := make(map[string]trans.Trans, len(destCmdTopics))
		for _, cmdTopic := range destCmdTopics {
			cmdTopic := cmdTopic // capture for closure
			destCmdTransMap[cmdTopic] = trans.NewSimpleTrans(func(topic string, payload []byte) error {
				if !isAllowedPayload(payload, rule) {
					return nil
				}
				// topic 格式為 dest.cmd_topic/<md5hex>，
				// 直接取出 suffix 接到 source.cmd_topic 即可，不需重新計算
				suffix := topic[len(rule.Dest.CmdTopic):]
				srcCmdTopic := rule.Source.CmdTopic + suffix
				return srcCmdPub.PublishToTopic(srcCmdTopic, payload)
			})
		}

		destSubCfg, err := makeMqttCfg(rule.Dest, destCmdTopics, "dest-sub-cmd-"+idxStr)
		if err != nil {
			return nil, err
		}
		destSubServLocal, err = mqtt.NewMqttSubOnlyServ(destSubCfg, destCmdTransMap)
		if err != nil {
			return nil, errors.Wrap(err, "create dest cmd subscriber fail")
		}
	}

	return &bridgeImpl{
		sourceSubServ: sourceSubServ,
		destSubServ:   destSubServLocal,
	}, nil
}

func (b *bridgeImpl) Run(ctx context.Context) {
	if b.destSubServ != nil {
		go b.destSubServ.Run(ctx)
	}
	b.sourceSubServ.Run(ctx)
}

// isAllowedPayload 嘗試從 payload 解析出 id 或 mac，
// 並檢查是否在允許清單內。
// 若 payload 不含相關欄位或解析失敗，則放行（不過濾）。
func isAllowedPayload(payload []byte, rule BridgeRule) bool {
	// 若裝置清單為空，沒有任何 device 要轉發 -> 不放行
	if len(rule.MacSet) == 0 {
		return false
	}

	var data map[string]interface{}
	// 若不是可解析的 JSON，視為無法識別裝置 -> 不放行
	if err := json.Unmarshal(payload, &data); err != nil {
		return false
	}

	// 必須包含 MAC_Address 與 GW_ID 欄位，且兩者要同時匹配到規則
	macVal, ok := data["MAC_Address"]
	if !ok {
		return false
	}
	gwVal, ok := data["GW_ID"]
	if !ok {
		return false
	}
	macStr, _ := macVal.(string)
	gwStr, _ := gwVal.(string)
	if macStr == "" || gwStr == "" {
		return false
	}

	// 檢查 mac 是否存在，且對應的 device id (GW_ID) 必須相同
	expectedGwID, exists := rule.MacSet[macStr]
	if !exists {
		return false
	}
	return expectedGwID == gwStr
}
