package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"jarvis/dao/db/elasticsearch"
	"jarvis/dao/db/mysql"
	"jarvis/dao/db/redis"
	"jarvis/logger"
	"jarvis/middleware/mq/nats"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/duke-git/lancet/datetime"
	ONats "github.com/nats-io/nats.go"
	ORedis "github.com/redis/go-redis/v9"
	"github.com/spf13/cast"
)

type AnalyzeRequest struct {
	UserID uint   `json:"user_id"`
	Text   string `json:"text"`
}

type PinCheck struct {
	UserID    int    `json:"user_id"`
	MessageID int    `json:"message_id"`
	ChatID    int    `json:"chat_id"`
	Username  string `json:"username"`
}

type PinRequest struct {
	UserID    int            `json:"user_id"`
	MessageID int            `json:"message_id"`
	ChatID    int            `json:"chat_id"`
	Content   string         `json:"content"`
	ParseMode string         `json:"parse_mode"`
	Markup    map[string]any `json:"markup"`
}

type MissionRequest struct {
	TraceID   string `json:"trace_id"`
	UserID    int    `json:"user_id"`
	MessageID int    `json:"message_id"`
	ChatID    int    `json:"chat_id"`
	Username  string `json:"username"`
	Behavior  string `json:"behavior"`
	Content   string `json:"content"`
}

type MissionResponse struct {
	Content   string         `json:"content"`
	ParseMode string         `json:"parse_mode"`
	Markup    map[string]any `json:"markup"`
	Error     string         `json:"error"`
	TraceID   string         `json:"trace_id"`
}

type DeleteRequest struct {
	MessageID int `json:"message_id"`
	ChatID    int `json:"chat_id"`
}

const (
	JSSearchCacheSubject = "Search.Cache"
	JSSearchImpSubject   = "Search.Impressions"
)

var (
	_subscriptions = make([]*ONats.Subscription, 0)
)

func checkAndPin() {
	logger.App().Infoln("=================================================== start check and pin ===================================================")
	defer logger.App().Infoln("=================================================== stop check and pin ===================================================")

	for {
		done := false

		select {
		case <-_close:
			{
				done = true
			}
		case request, ok := <-_check:
			{
				if !ok {
					done = true
					break
				}

				if request.UserID == 0 {
					continue
				}

				logger.App().Infof("=================== receive user id : %+v", request)

				key := fmt.Sprintf("user:action:%d", request.UserID)

				if cmd := redis.Instance().IncrBy(context.Background(), key, 1); cmd.Err() != nil {
					logger.App().Errorf("incr %s error : %s", key, cmd.Err().Error())
					continue
				} else {
					if (cmd.Val() == 1) || ((cmd.Val() % 25) == 0) {
						logger.App().Infof("[%d] user action [%d] , send a pin", request.UserID, cmd.Val())

						if title, link := GetTypeAd(request.Username, 2); title != "" && link != "" {
							response := &SSMResponseMsg{
								Type:    RTPin,
								TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: request.InMsgID, OutMsgID: request.OutMsgID,
								Content:   title,
								ParseMode: ParseModeText,
								Markup:    generateMarkup([][][]string{{{"点击畅享", link, ""}}}),
							}
							if err := doSendSSMResponse(response); err != nil {
								logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
								continue
							}
						}

					}
				}
			}
		}

		if done {
			break
		}
	}
}

func parseAndStore() {
	logger.App().Infoln("=================================================== start do word analyze ===================================================")
	defer logger.App().Infoln("=================================================== stop do word analyze ===================================================")

	for {
		done := false

		select {
		case <-_close:
			{
				done = true
			}
		case request, ok := <-_channel:
			{
				if !ok {
					done = true
					break
				}

				logger.App().Infof("=================== receive analyze : %+v", request)

				if request.UserID != 0 {
					now := time.Now()

					isNew := false

					inTotal := true
					if cmd := redis.Instance().Get(context.Background(), fmt.Sprintf("UserID:%d", request.UserID)); cmd.Err() != nil {
						if cmd.Err() != ORedis.Nil {
							logger.App().Errorf("get [%s] error: %s", fmt.Sprintf("UserID:%d", request.UserID), cmd.Err().Error())
						}
						inTotal = false
					}

					if !inTotal {
						isNew = true
						if err := redis.Instance().Set(context.Background(), fmt.Sprintf("UserID:%d", request.UserID), 0, 0).Err(); err != nil {
							logger.App().Errorf("set [%s] error: %s", fmt.Sprintf("UserID:%d", request.UserID), err.Error())
						}
						if err := redis.Instance().SAdd(context.Background(), fmt.Sprintf("%s:TodayNewUser:SET", now.Format("20060102")), request.UserID).Err(); err != nil {
							logger.App().Errorf("sadd [%s] error: %s", fmt.Sprintf("UserID:%d", request.UserID), err.Error())
						}
					} else {
						inToday := false
						if cmd := redis.Instance().SIsMember(context.Background(), fmt.Sprintf("%s:TodayNewUser:SET", now.Format("20060102")), request.UserID); cmd.Err() != nil {
							logger.App().Errorf("get [%s] error: %s", fmt.Sprintf("UserID:%d", request.UserID), cmd.Err().Error())
						} else {
							inToday = cmd.Val()
						}

						if inToday {
							isNew = true
						}
					}

					// total use
					if cmd := redis.Instance().IncrBy(context.Background(), "TotalUse", 1); cmd.Err() != nil {
						logger.App().Error("incr error : ", cmd.Err().Error())
					}
					// today use
					if cmd := redis.Instance().IncrBy(context.Background(), fmt.Sprintf("%s:TodayUse", now.Format("20060102")), 1); cmd.Err() != nil {
						logger.App().Error("incr error : ", cmd.Err().Error())
					}
					// hour use
					if cmd := redis.Instance().IncrBy(context.Background(), fmt.Sprintf("%s:Use", now.Format("2006010215")), 1); cmd.Err() != nil {
						logger.App().Error("incr error : ", cmd.Err().Error())
					}
					// total user
					if cmd := redis.Instance().PFAdd(context.Background(), "TotalUser", request.UserID); cmd.Err() != nil {
						logger.App().Error("pfadd error : ", cmd.Err().Error())
					}
					// today user
					if cmd := redis.Instance().PFAdd(context.Background(), fmt.Sprintf("%s:TodayUser", now.Format("20060102")), request.UserID); cmd.Err() != nil {
						logger.App().Error("pfadd error : ", cmd.Err().Error())
					}
					if isNew {
						// today new user use
						if cmd := redis.Instance().IncrBy(context.Background(), fmt.Sprintf("%s:TodayNewUserUse", now.Format("20060102")), 1); cmd.Err() != nil {
							logger.App().Error("incr error : ", cmd.Err().Error())
						}
						// today new user
						if cmd := redis.Instance().PFAdd(context.Background(), fmt.Sprintf("%s:TodayNewUser", now.Format("20060102")), request.UserID); cmd.Err() != nil {
							logger.App().Error("pfadd error : ", cmd.Err().Error())
						}
					}
				}

				// 构建分析请求体
				reqBody := map[string]any{"analyzer": "ik_smart", "text": request.Content}

				// 编码成 JSON
				body, err := json.Marshal(reqBody)
				if err != nil {
					logger.App().Errorf("Error encoding request: %s", err.Error())
					continue
				}

				// 执行请求
				res, err := elasticsearch.Instance().Indices.Analyze(
					elasticsearch.Instance().Indices.Analyze.WithBody(bytes.NewReader(body)),
					elasticsearch.Instance().Indices.Analyze.WithContext(context.Background()),
				)
				if err != nil {
					logger.App().Errorf("Analyze request failed: %s", err.Error())
					continue
				}
				defer res.Body.Close()

				// 检查状态
				if res.IsError() {
					logger.App().Errorf("Error analyzing text: %s", res.String())
				}

				// 解析响应
				var result struct {
					Tokens []struct {
						Token string `json:"token"`
					} `json:"tokens"`
				}
				if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
					logger.App().Errorf("Error parsing response: %s", err.Error())
					continue
				}

				m := make(map[string]struct{})
				for _, t := range result.Tokens {
					if len([]rune(t.Token)) < 2 {
						continue
					}

					m[t.Token] = struct{}{}
				}

				for token := range m {
					if err := redis.Instance().ZIncrBy(context.Background(), "HotRankList", 1.0, token).Err(); err != nil {
						logger.App().Error("zincrby %s error : %s", token, err.Error())
					}
					if err := redis.Instance().HIncrBy(context.Background(), "SearchHash", token, 1).Err(); err != nil {
						logger.App().Error("hincrby %s error : %s", token, err.Error())
					}
					if err := mysql.Instance().Table("search_log").Create(&struct {
						ID      uint   `gorm:"column:id;not null;autoIncrement:false;primaryKey;comment:主键ID" json:"id"`
						Created int64  `gorm:"column:created;not null;index:idx_create_time;index:idx_word_create_time,priority:2;comment:时间戳(毫秒);primaryKey" json:"created"`
						Word    string `gorm:"column:word;type:varchar(255);not null;index:idx_word_create_time,priority:1;comment:搜索词" json:"word"`
					}{
						Created: time.Now().UnixMilli(),
						Word:    token,
					}).Error; err != nil {
						logger.App().Error("insert %s error : %s", token, err.Error())
					}
				}
			}
		}

		if done {
			break
		}
	}
}

func flushRankList() {
	logger.App().Infoln("=================================================== start flush rank list ===================================================")
	defer logger.App().Infoln("=================================================== stop flush rank list ===================================================")

	ticker := time.NewTicker(time.Second * time.Duration(1))
	defer ticker.Stop()

	for {
		done := false

		select {
		case <-_close:
			{
				done = true
			}
		case t, ok := <-ticker.C:
			{
				if !ok {
					done = true
					break
				}

				hour, minute, second := t.Hour(), t.Minute(), t.Second()

				// redis corn
				if minute == 0 && second == 0 {

					script := `
local key = KEYS[1]
local factor = tonumber(ARGV[1])
local members = redis.call("ZRANGE", key, 0, -1, "WITHSCORES")
for i = 1, #members, 2 do
    local member = members[i]
    local score = tonumber(members[i+1])
    local newScore = score * factor
    redis.call("ZADD", key, newScore, member)
end
return true
`
					if err := redis.Instance().Eval(context.Background(), script, []string{"HotRankList"}, fmt.Sprintf("%f", 0.98)).Err(); err != nil {
						logger.App().Errorf("decay error : %s", err.Error())
					}

					yesterday := t.Add(time.Hour * time.Duration(-24))
					keys := []string{
						fmt.Sprintf("%s:Use", yesterday.Format("2006010215")),            // hour use
						fmt.Sprintf("%s:TodayUse", yesterday.Format("20060102")),         // today use
						fmt.Sprintf("%s:TodayUser", yesterday.Format("20060102")),        // today user
						fmt.Sprintf("%s:TodayNewUserUse", yesterday.Format("20060102")),  // today new user use
						fmt.Sprintf("%s:TodayNewUser", yesterday.Format("20060102")),     // today new user
						fmt.Sprintf("%s:TodayNewUser:SET", yesterday.Format("20060102")), // today new user set
					}
					if err := redis.Instance().Del(context.Background(), keys...).Err(); err != nil && err != ORedis.Nil {
						logger.App().Errorf("dele keys %+v error : %s", keys, err.Error())
					}
				}

				// mysql corn
				if hour == 0 && minute == 0 && second == 5 {
					standard := time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local)

					now := time.Now().AddDate(0, 0, 2)

					todayZero := datetime.BeginOfDay(now)

					days := int(todayZero.Sub(standard).Hours() / 24)
					left := days % 10

					nowOn := now.Add(time.Hour * time.Duration(-left))

					for i := 0; i < 6; i++ {
						start := nowOn.AddDate(0, 0, i*10)
						end := nowOn.AddDate(0, 0, ((i+1)*10)-1)

						key := fmt.Sprintf("p%s_%s", start.Format("20060102"), end.Format("20060102"))

						endYear, endMonth, endDay := end.AddDate(0, 0, 1).Date()
						lessThan := time.Date(endYear, endMonth, endDay, 0, 0, 0, 0, end.Location())

						sql1 := fmt.Sprintf(`ALTER TABLE search_log ADD PARTITION (PARTITION %s VALUES LESS THAN (%d))`, key, lessThan.UnixMilli())
						sql2 := fmt.Sprintf(`ALTER TABLE ad_log ADD PARTITION (PARTITION %s VALUES LESS THAN (%d))`, key, lessThan.UnixMilli())
						if err := mysql.Instance().Exec(sql1).Error; err != nil {
							logger.App().Errorf("exec error : %s - %s", err.Error(), sql1)
						}
						if err := mysql.Instance().Exec(sql2).Error; err != nil {
							logger.App().Errorf("exec error : %s - %s", err.Error(), sql2)
						}
					}
				}
			}
		}

		if done {
			break
		}
	}
}

var markdownV2SpecialRunes = map[rune]string{
	'_':  `\_`,
	'*':  `\*`,
	'[':  `\[`,
	']':  `\]`,
	'(':  `\(`,
	')':  `\)`,
	'~':  `\~`,
	'`':  "\\`",
	'>':  `\>`,
	'#':  `\#`,
	'+':  `\+`,
	'-':  `\-`,
	'=':  `\=`,
	'|':  `\|`,
	'{':  `\{`,
	'}':  `\}`,
	'.':  `\.`,
	'!':  `\!`,
	'\\': `\\`,
}

func EscapeMarkdownV2(text string) string {
	var b strings.Builder
	b.Grow(len(text) * 2)

	for _, r := range text {
		if esc, ok := markdownV2SpecialRunes[r]; ok {
			b.WriteString(esc)
		} else {
			b.WriteRune(r)
		}
	}

	return b.String()
}

func doCache(msg *ONats.Msg) {
	t := string(msg.Data)

	switch t {
	case "1":
		{
			if err := loadKeyword(); err != nil {
				logger.App().Errorf("load keyword error : %s", err.Error())
			}
		}
	case "2":
		{
			if err := loadAd(); err != nil {
				logger.App().Errorf("load ad error : %s", err.Error())
			}
		}
	case "3":
		{
			if err := loadKeywordAd(); err != nil {
				logger.App().Errorf("load keyword ad error : %s", err.Error())
			}
		}
	}
}

func doImpressions(msg *ONats.Msg) {
	aid := cast.ToUint(string(msg.Data))
	doADImpressions(aid)
}

// ========================================================================================================

const (
	RKVideoFileID = "VideoFileIDMap"

	VideoCL      = "changeLanuage.mp4"
	VideoFMV     = "freeMusicDesc.mp4"
	VideoR18Desc = "release18Desc.mp4"
	VideoBuildG  = "buildGroupDesc.mp4"
	VideoStart   = "start.mp4"
)

var _videoFileIDMap = map[string]string{}

func loadVideMapCache() error {

	if cmd := redis.Instance().HGetAll(context.Background(), RKVideoFileID); cmd.Err() != nil {
		return cmd.Err()
	} else {
		for k, v := range cmd.Val() {
			_videoFileIDMap[k] = v
		}
	}

	logger.App().Infof("======= load video map success : %+v", _videoFileIDMap)

	return nil
}

// =====================================================================================================================================

type SSMResponseType uint16

const (
	RTEdit   SSMResponseType = iota // reply to edit
	RTSend                          // just send
	RTPin                           // send a new message then pin it
	RTDelete                        // delete that message
	RTVideo                         // send a new message with video
)

const (
	SSQueue                  = "SearchQueue"
	SSMissionRequestSubject  = "Search.Mission.Request"
	SSMissionResponseSubject = "Search.Mission.Response"
)

type SSMRequestMsg struct {
	TraceID  string `json:"trace_id"`
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	FLName   string `json:"fl_name"`
	ChatID   int    `json:"chat_id"`
	InMsgID  int    `json:"in_msg_id"`
	OutMsgID int    `json:"out_msg_id"`
	Behavior string `json:"behavior"`
	Content  string `json:"content"`
}

type SSMResponseMsg struct {
	Type        SSMResponseType `json:"type"`
	TraceID     string          `json:"trace_id"`
	UserID      int             `json:"user_id"`
	Username    string          `json:"username"`
	ChatID      int             `json:"chat_id"`
	InMsgID     int             `json:"in_msg_id"`
	OutMsgID    int             `json:"out_msg_id"`
	Content     string          `json:"content"`
	ParseMode   string          `json:"parse_mode"`
	Markup      map[string]any  `json:"markup"`
	VideoFileID string          `json:"video_file_id"`
	Error       string          `json:"error"`
}

func doRequest(msg *ONats.Msg) {
	request := new(SSMRequestMsg)

	logger.App().Infof("=================================== request : %s", string(msg.Data))

	if err := sonic.Unmarshal(msg.Data, request); err != nil {
		logger.App().Errorf("unmarshal error : %s - %s", err.Error())
		return
	}

	// any behavior touch the pin check
	go func() { _check <- *(request) }()

	standard := request.Behavior
	if request.Behavior == "" {
		standard = request.Content
	}

	handler, exist := _behaviorMap[standard]
	if exist {
		go handler(request)
	} else {
		go handleOther(request)
	}
}

func doSendSSMResponse(response *SSMResponseMsg) error {
	if response == nil {
		return errors.New("response is nil")
	}

	data, err := sonic.Marshal(response)
	if err != nil {
		return err
	}

	logger.App().Infof("=================================== response : %s", string(data))

	return nats.Instance().Publish(SSMissionResponseSubject, data)
}

func handleStart(request *SSMRequestMsg) {
	if fileID, exist := _videoFileIDMap[VideoStart]; exist {
		extraResponse := &SSMResponseMsg{
			Type:    RTVideo,
			TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: request.InMsgID, OutMsgID: request.OutMsgID,
			Content:   "🔍我是个资源搜索引擎，向我发送关键词来寻找群组、频道、视频、音乐、[中文包](https://t.me/setlanguage/zh-hans-beta)",
			ParseMode: ParseModeMarkdownV2,
			Markup: map[string]any{
				"keyboard":                [][]*KeyboardButton{{{Text: OrderCDaoh}, {Text: OrderCReso}}}, // 自定义按钮的二维数组
				"is_persistent":           true,                                                          // 请求客户端在隐藏系统键盘时仍显示自定义键盘
				"resize_keyboard":         true,                                                          // 请求客户端根据内容调整键盘高度
				"input_field_placeholder": "搜你所爱",                                                        // 激活键盘时输入框显示的占位符文本
			},
			VideoFileID: fileID,
		}
		if err := doSendSSMResponse(extraResponse); err != nil {
			logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(extraResponse))
		}
	}

	response := &SSMResponseMsg{
		Type:    RTSend,
		TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: 0, OutMsgID: request.OutMsgID,
		Content:   "",
		ParseMode: "",
		Markup:    map[string]any{},
	}
	if content, parseMode, markup, err := reso(); err != nil {
		response.Error = err.Error()
		logger.App().Errorf("[%s] parse error : %s", response.TraceID, err.Error())
	} else {
		response.Content = content
		response.ParseMode = parseMode
		response.Markup = markup
	}

	if err := doSendSSMResponse(response); err != nil {
		logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
	}
}

func handleReso(request *SSMRequestMsg) {
	response := &SSMResponseMsg{
		Type:    RTSend,
		TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: request.InMsgID, OutMsgID: request.OutMsgID,
		Content:   "",
		ParseMode: "",
		Markup:    map[string]any{},
	}

	if content, parseMode, markup, err := reso(); err != nil {
		response.Error = err.Error()
		logger.App().Errorf("[%s] parse error : %s", response.TraceID, err.Error())
	} else {
		response.Content = content
		response.ParseMode = parseMode
		response.Markup = markup
	}

	if err := doSendSSMResponse(response); err != nil {
		logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
	}
}

func handleDaoh(request *SSMRequestMsg) {
	response := &SSMResponseMsg{
		Type:    RTSend,
		TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: request.InMsgID, OutMsgID: request.OutMsgID,
		Content:   "",
		ParseMode: "",
		Markup:    map[string]any{},
	}

	if content, parseMode, markup, err := daoh(); err != nil {
		response.Error = err.Error()
		logger.App().Errorf("[%s] parse error : %s", response.TraceID, err.Error())
	} else {
		response.Content = content
		response.ParseMode = parseMode
		response.Markup = markup
	}

	if err := doSendSSMResponse(response); err != nil {
		logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
	}
}

func handleHelp(request *SSMRequestMsg) {
	response := &SSMResponseMsg{
		Type:    RTSend,
		TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: 0, OutMsgID: request.OutMsgID,
		Content:   "",
		ParseMode: "",
		Markup:    map[string]any{},
	}

	if request.Behavior == BehaviorBack {
		response.Type = RTEdit
	}

	if content, parseMode, markup, err := help(); err != nil {
		response.Error = err.Error()
		logger.App().Errorf("[%s] parse error : %s", response.TraceID, err.Error())
	} else {
		response.Content = content
		response.ParseMode = parseMode
		response.Markup = markup
	}

	if err := doSendSSMResponse(response); err != nil {
		logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
	}
}

func handleMore(request *SSMRequestMsg) {
	response := &SSMResponseMsg{
		Type:    RTSend,
		TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: request.InMsgID, OutMsgID: request.OutMsgID,
		Content:   "",
		ParseMode: "",
		Markup:    map[string]any{},
	}

	if request.Content == "" {
		response.Type = RTEdit
	}

	if content, parseMode, markup, err := more(); err != nil {
		response.Error = err.Error()
		logger.App().Errorf("[%s] parse error : %s", response.TraceID, err.Error())
	} else {
		response.Content = content
		response.ParseMode = parseMode
		response.Markup = markup
	}

	if err := doSendSSMResponse(response); err != nil {
		logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
	}
}

func handlePrivacy(request *SSMRequestMsg) {
	response := &SSMResponseMsg{
		Type:    RTSend,
		TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: request.InMsgID, OutMsgID: request.OutMsgID,
		Content:   "",
		ParseMode: "",
		Markup:    map[string]any{},
	}

	if content, parseMode, markup, err := privacy(); err != nil {
		response.Error = err.Error()
		logger.App().Errorf("[%s] parse error : %s", response.TraceID, err.Error())
	} else {
		response.Content = content
		response.ParseMode = parseMode
		response.Markup = markup
	}

	if err := doSendSSMResponse(response); err != nil {
		logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
	}
}

func handleOther(request *SSMRequestMsg) {
	response := &SSMResponseMsg{
		Type:    RTSend,
		TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: request.InMsgID, OutMsgID: request.OutMsgID,
		Content:   "",
		ParseMode: "",
		Markup:    map[string]any{},
	}

	if request.Behavior != "" {
		response.Type = RTEdit
	}

	if strings.HasPrefix(request.Content, OrderStart) {
		value := strings.Trim(request.Content, fmt.Sprintf("%s ", OrderStart))
		if v, ok := _resoMap.Load(value); ok {
			request.Content = cast.ToString(v)
		} else {
			request.Content = value
		}
	}

	// only do analyze when send a search
	go func() { _channel <- *(request) }()

	if content, parseMode, markup, err := other(request.UserID, request.InMsgID, request.Username, request.Behavior, request.Content); err != nil {
		response.Error = err.Error()
		logger.App().Errorf("[%s] parse error : %s", response.TraceID, err.Error())
	} else {
		response.Content = content
		response.ParseMode = parseMode
		response.Markup = markup
	}

	if err := doSendSSMResponse(response); err != nil {
		logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
	}
}

func handleDaohAnother(request *SSMRequestMsg) {
	response := &SSMResponseMsg{
		Type:    RTSend,
		TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: request.InMsgID, OutMsgID: request.OutMsgID,
		Content:   "",
		ParseMode: "",
		Markup:    map[string]any{},
	}

	response.Type = RTEdit

	if content, parseMode, markup, err := more(); err != nil {
		response.Error = err.Error()
		logger.App().Errorf("[%s] parse error : %s", response.TraceID, err.Error())
	} else {
		response.Content = content
		response.ParseMode = parseMode
		response.Markup = markup
	}

	if err := doSendSSMResponse(response); err != nil {
		logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
	}
}

func handleDaohBack(request *SSMRequestMsg) {
	response := &SSMResponseMsg{
		Type:    RTSend,
		TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: request.InMsgID, OutMsgID: request.OutMsgID,
		Content:   "",
		ParseMode: "",
		Markup:    map[string]any{},
	}

	response.Type = RTEdit

	if content, parseMode, markup, err := more(); err != nil {
		response.Error = err.Error()
		logger.App().Errorf("[%s] parse error : %s", response.TraceID, err.Error())
	} else {
		response.Content = content
		response.ParseMode = parseMode
		response.Markup = markup
	}

	if err := doSendSSMResponse(response); err != nil {
		logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
	}
}

func handleHelpR18(request *SSMRequestMsg) {
	response := &SSMResponseMsg{
		Type:    RTSend,
		TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: request.InMsgID, OutMsgID: request.OutMsgID,
		Content:   "",
		ParseMode: "",
		Markup:    map[string]any{},
	}

	response.Type = RTVideo

	response.Content, response.ParseMode, response.Markup, response.VideoFileID = behaviorRelease18()

	if err := doSendSSMResponse(response); err != nil {
		logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
	}
}

func handleHelpFM(request *SSMRequestMsg) {
	response := &SSMResponseMsg{
		Type:    RTSend,
		TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: request.InMsgID, OutMsgID: request.OutMsgID,
		Content:   "",
		ParseMode: "",
		Markup:    map[string]any{},
	}

	response.Type = RTVideo

	response.Content, response.ParseMode, response.Markup, response.VideoFileID = behaviorFreeMusic()

	if err := doSendSSMResponse(response); err != nil {
		logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
	}
}

func handleHelpCL(request *SSMRequestMsg) {
	response := &SSMResponseMsg{
		Type:    RTSend,
		TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: request.InMsgID, OutMsgID: request.OutMsgID,
		Content:   "",
		ParseMode: "",
		Markup:    map[string]any{},
	}

	response.Type = RTVideo

	response.Content, response.ParseMode, response.Markup, response.VideoFileID = behaviorChangeLanuage()

	if err := doSendSSMResponse(response); err != nil {
		logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
	}
}

func handleHelpDS(request *SSMRequestMsg) {
	response := &SSMResponseMsg{
		Type:    RTSend,
		TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: request.InMsgID, OutMsgID: request.OutMsgID,
		Content:   "",
		ParseMode: "",
		Markup:    map[string]any{},
	}

	response.InMsgID = 0

	response.Content, response.ParseMode, response.Markup = behaviorDefendScam()

	if err := doSendSSMResponse(response); err != nil {
		logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
	}
}

func handleHelpRMG(request *SSMRequestMsg) {
	response := &SSMResponseMsg{
		Type:    RTSend,
		TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: request.InMsgID, OutMsgID: request.OutMsgID,
		Content:   "",
		ParseMode: "",
		Markup:    map[string]any{},
	}

	response.InMsgID = 0

	response.Content, response.ParseMode, response.Markup = behaviorRecordMyGroup()

	if err := doSendSSMResponse(response); err != nil {
		logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
	}
}

func handleHelpBG(request *SSMRequestMsg) {
	response := &SSMResponseMsg{
		Type:    RTSend,
		TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: request.InMsgID, OutMsgID: request.OutMsgID,
		Content:   "",
		ParseMode: "",
		Markup:    map[string]any{},
	}

	response.Type = RTVideo

	response.Content, response.ParseMode, response.Markup, response.VideoFileID = behaviorBuildGroup()

	if err := doSendSSMResponse(response); err != nil {
		logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
	}
}

func handleHelpProfit(request *SSMRequestMsg) {
	response := &SSMResponseMsg{
		Type:    RTSend,
		TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: request.InMsgID, OutMsgID: request.OutMsgID,
		Content:   "",
		ParseMode: "",
		Markup:    map[string]any{},
	}

	response.Type = RTEdit

	response.Content, response.ParseMode, response.Markup = behaviorProfit()

	if err := doSendSSMResponse(response); err != nil {
		logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
	}
}

func handleHelpAD(request *SSMRequestMsg) {
	response := &SSMResponseMsg{
		Type:    RTSend,
		TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: request.InMsgID, OutMsgID: request.OutMsgID,
		Content:   "",
		ParseMode: "",
		Markup:    map[string]any{},
	}

	response.Type = RTEdit

	response.Content, response.ParseMode, response.Markup = behaviorAD()

	if err := doSendSSMResponse(response); err != nil {
		logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
	}
}

func handleHelpReport(request *SSMRequestMsg) {
	response := &SSMResponseMsg{
		Type:    RTSend,
		TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: request.InMsgID, OutMsgID: request.OutMsgID,
		Content:   "",
		ParseMode: "",
		Markup:    map[string]any{},
	}

	response.Type = RTEdit

	response.Content, response.ParseMode, response.Markup = behaviorReport()

	if err := doSendSSMResponse(response); err != nil {
		logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
	}
}

func handleMoreShowQuery(request *SSMRequestMsg) {
	response := &SSMResponseMsg{
		Type:    RTSend,
		TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: request.InMsgID, OutMsgID: request.OutMsgID,
		Content:   "",
		ParseMode: "",
		Markup:    map[string]any{},
	}

	response.InMsgID = 0

	response.Content, response.ParseMode, response.Markup = behaviorShowQuery()

	if err := doSendSSMResponse(response); err != nil {
		logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
	}
}

func handleMoreR18(request *SSMRequestMsg) {
	response := &SSMResponseMsg{
		Type:    RTSend,
		TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: request.InMsgID, OutMsgID: request.OutMsgID,
		Content:   "",
		ParseMode: "",
		Markup:    map[string]any{},
	}

	response.Type = RTVideo

	response.Content, response.ParseMode, response.Markup, response.VideoFileID = behaviorRelease18()

	if err := doSendSSMResponse(response); err != nil {
		logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
	}
}

func handleMoreRML(request *SSMRequestMsg) {
	response := &SSMResponseMsg{
		Type:    RTSend,
		TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: request.InMsgID, OutMsgID: request.OutMsgID,
		Content:   "",
		ParseMode: "",
		Markup:    map[string]any{},
	}

	response.InMsgID = 0

	response.Content, response.ParseMode, response.Markup = behaviorRecordMyLink()

	if err := doSendSSMResponse(response); err != nil {
		logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
	}
}

func handleMoreIMM(request *SSMRequestMsg) {
	response := &SSMResponseMsg{
		Type:    RTSend,
		TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: 0, OutMsgID: request.OutMsgID,
		Content:   "",
		ParseMode: "",
		Markup:    map[string]any{},
	}

	if request.Content == "" {
		response.Type = RTEdit
	}

	response.Content, response.ParseMode, response.Markup = behaviorInviteMakeMoney(request.UserID, request.FLName)

	if err := doSendSSMResponse(response); err != nil {
		logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
	}
}

func handleMorePAD(request *SSMRequestMsg) {
	response := &SSMResponseMsg{
		Type:    RTSend,
		TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: 0, OutMsgID: request.OutMsgID,
		Content:   "",
		ParseMode: "",
		Markup:    map[string]any{},
	}

	if request.Content == "" {
		response.Type = RTEdit
	}

	response.Content, response.ParseMode, response.Markup = behaviorPutAD()

	if err := doSendSSMResponse(response); err != nil {
		logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
	}
}

func handleMorePT(request *SSMRequestMsg) {
	response := &SSMResponseMsg{
		Type:    RTSend,
		TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: 0, OutMsgID: request.OutMsgID,
		Content:   "",
		ParseMode: "",
		Markup:    map[string]any{},
	}

	response.Content, response.ParseMode, response.Markup = behaviorPT(request.UserID)

	if err := doSendSSMResponse(response); err != nil {
		logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
	}
}

func handleMoreCQ(request *SSMRequestMsg) {
	response := &SSMResponseMsg{
		Type:    RTEdit,
		TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: request.InMsgID, OutMsgID: request.OutMsgID,
		Content:   "",
		ParseMode: "",
		Markup:    map[string]any{},
	}

	response.Content, response.ParseMode, response.Markup = behaviorCQ()

	if err := doSendSSMResponse(response); err != nil {
		logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
	}
}

func handlePrivacyClose(request *SSMRequestMsg) {
	response := &SSMResponseMsg{
		Type:    RTSend,
		TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: request.InMsgID, OutMsgID: request.OutMsgID,
		Content:   "",
		ParseMode: "",
		Markup:    map[string]any{},
	}

	response.Type = RTDelete

	if err := doSendSSMResponse(response); err != nil {
		logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
	}
}

func handleMorePADKW(request *SSMRequestMsg) {
	response := &SSMResponseMsg{
		Type:    RTEdit,
		TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: request.InMsgID, OutMsgID: request.OutMsgID,
		Content:   "",
		ParseMode: "",
		Markup:    map[string]any{},
	}

	response.Content, response.ParseMode, response.Markup = behaviorPADKW()

	if err := doSendSSMResponse(response); err != nil {
		logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
	}
}

func handleMorePADTL(request *SSMRequestMsg) {
	response := &SSMResponseMsg{
		Type:    RTEdit,
		TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: request.InMsgID, OutMsgID: request.OutMsgID,
		Content:   "",
		ParseMode: "",
		Markup:    map[string]any{},
	}

	response.Content, response.ParseMode, response.Markup = behaviorPADTL()

	if err := doSendSSMResponse(response); err != nil {
		logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
	}
}

func handleMorePADBL(request *SSMRequestMsg) {
	response := &SSMResponseMsg{
		Type:    RTEdit,
		TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: request.InMsgID, OutMsgID: request.OutMsgID,
		Content:   "",
		ParseMode: "",
		Markup:    map[string]any{},
	}

	response.Content, response.ParseMode, response.Markup = behaviorPADBL()

	if err := doSendSSMResponse(response); err != nil {
		logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
	}
}

func handleMorePADGP(request *SSMRequestMsg) {
	response := &SSMResponseMsg{
		Type:    RTEdit,
		TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: request.InMsgID, OutMsgID: request.OutMsgID,
		Content:   "",
		ParseMode: "",
		Markup:    map[string]any{},
	}

	response.Content, response.ParseMode, response.Markup = behaviorPADGP()

	if err := doSendSSMResponse(response); err != nil {
		logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
	}
}

func handleMorePADBAD(request *SSMRequestMsg) {
	response := &SSMResponseMsg{
		Type:    RTEdit,
		TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: request.InMsgID, OutMsgID: request.OutMsgID,
		Content:   "",
		ParseMode: "",
		Markup:    map[string]any{},
	}

	response.Content, response.ParseMode, response.Markup = behaviorPADBAD()

	if err := doSendSSMResponse(response); err != nil {
		logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
	}
}

func handleMorePADHPAD(request *SSMRequestMsg) {
	response := &SSMResponseMsg{
		Type:    RTEdit,
		TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: request.InMsgID, OutMsgID: request.OutMsgID,
		Content:   "",
		ParseMode: "",
		Markup:    map[string]any{},
	}

	response.Content, response.ParseMode, response.Markup = behaviorPADHPAD()

	if err := doSendSSMResponse(response); err != nil {
		logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
	}
}

func handleMorePADMAD(request *SSMRequestMsg) {
	response := &SSMResponseMsg{
		Type:    RTEdit,
		TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: request.InMsgID, OutMsgID: request.OutMsgID,
		Content:   "",
		ParseMode: "",
		Markup:    map[string]any{},
	}

	response.Content, response.ParseMode, response.Markup = behaviorPADMAD(request.UserID, request.Username, request.FLName)

	if err := doSendSSMResponse(response); err != nil {
		logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
	}
}

func handleMoreIMMPF(request *SSMRequestMsg) {
	response := &SSMResponseMsg{
		Type:    RTEdit,
		TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: request.InMsgID, OutMsgID: request.OutMsgID,
		Content:   "",
		ParseMode: "",
		Markup:    map[string]any{},
	}

	response.Content, response.ParseMode, response.Markup = behaviorIMMPF()

	if err := doSendSSMResponse(response); err != nil {
		logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
	}
}

func handleMoreIMMPCO(request *SSMRequestMsg) {
	response := &SSMResponseMsg{
		Type:    RTEdit,
		TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: request.InMsgID, OutMsgID: request.OutMsgID,
		Content:   "",
		ParseMode: "",
		Markup:    map[string]any{},
	}

	response.Content, response.ParseMode, response.Markup = behaviorIMMPCO()

	if err := doSendSSMResponse(response); err != nil {
		logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
	}
}

func handleMoreIMMGNR(request *SSMRequestMsg) {
	response := &SSMResponseMsg{
		Type:    RTSend,
		TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: request.InMsgID, OutMsgID: request.OutMsgID,
		Content:   "",
		ParseMode: "",
		Markup:    map[string]any{},
	}

	response.Content, response.ParseMode, response.Markup = behaviorIMMGNR()

	if err := doSendSSMResponse(response); err != nil {
		logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
	}
}

func handleMoreIMMPR(request *SSMRequestMsg) {
	response := &SSMResponseMsg{
		Type:    RTSend,
		TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: request.InMsgID, OutMsgID: request.OutMsgID,
		Content:   "",
		ParseMode: "",
		Markup:    map[string]any{},
	}

	response.Content, response.ParseMode, response.Markup = behaviorIMMPR()

	if err := doSendSSMResponse(response); err != nil {
		logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
	}
}

func handleMoreIMMBA(request *SSMRequestMsg) {
	response := &SSMResponseMsg{
		Type:    RTSend,
		TraceID: request.TraceID, UserID: request.UserID, Username: request.Username, ChatID: request.ChatID, InMsgID: 0, OutMsgID: request.OutMsgID,
		Content:   "",
		ParseMode: "",
		Markup:    map[string]any{},
	}

	response.Content, response.ParseMode, response.Markup = behaviorIMMBA()

	if err := doSendSSMResponse(response); err != nil {
		logger.App().Errorf("do send SSMResponseMsg error : %s - %+v", err.Error(), *(response))
	}
}
