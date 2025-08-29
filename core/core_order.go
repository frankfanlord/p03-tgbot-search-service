package core

import (
	"context"
	"fmt"
	"jarvis/dao/db/redis"
	"jarvis/logger"
	"search-service/core/search"
	"strconv"
	"strings"
	"sync"

	"github.com/bytedance/sonic"
	"github.com/spf13/cast"
)

type BotCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

type KeyboardButton struct {
	Text string `json:"text"`
}

type InlineKeyboardButton struct {
	Text         string `json:"text"`
	Url          string `json:"url"`
	CallbackData string `json:"callback_data"`
}

const (
	ParseModeText       = ""
	ParseModeMarkdownV2 = "MarkdownV2"
	ParseModeHTML       = "HTML"
	ParseModeMarkdown   = "Markdown"
)

const (
	OrderStart   = "/start"
	OrderReso    = "/reso"
	OrderDaoh    = "/daoh"
	OrderHelp    = "/help"
	OrderMore    = "/more"
	OrderPrivacy = "/privacy"
	OrderCDaoh   = "👥群组导航"
	OrderCReso   = "🔍热搜排行"
)

const (
	BehaviorAll     = "🔄"
	BehaviorGroup   = "👥"
	BehaviorChannel = "📢"
	BehaviorVideo   = "🎬"
	BehaviorPhoto   = "🖼️"
	BehaviorVoice   = "🎧"
	BehaviorText    = "💬"
	BehaviorFile    = "📁"
	BehaviorBot     = "🤖"
	BehaviorLast    = "⬅️"
	BehaviorNext    = "➡️"
)

const (
	BehaviorDataAll     = "_ALL_"
	BehaviorDataGroup   = "_GROUP_"
	BehaviorDataChannel = "_CHANNEL_"
	BehaviorDataVideo   = "_VIDEO_"
	BehaviorDataPhoto   = "_PHOTO_"
	BehaviorDataVoice   = "_VOICE_"
	BehaviorDataText    = "_TEXT_"
	BehaviorDataFile    = "_FILE_"
	BehaviorDataBot     = "_BOT_"
	BehaviorDataLast    = "_LAST_"
	BehaviorDataNext    = "_NEXT_"

	BehaviorClose = "_CLOSE_"

	BehaviorBack    = "_BACK_"
	BehaviorAnother = "_ANOTHER_"

	BehaviorR18    = "_R18_"
	BehaviorFM     = "_RM_"
	BehaviorCL     = "_CL_"
	BehaviorDS     = "_DS_"
	BehaviorRMG    = "_RMG_"
	BehaviorBG     = "_BG_"
	BehaviorProfit = "_PROFIT_"
	BehaviorAD     = "_AD_"
	BehaviorReport = "_REPORT_"

	BehaviorShowQuery = "_SQ_"
	BehaviorRML       = "_RML_"

	BehaviorIMM = "_IMM_"
	BehaviorPAD = "_PAD_"

	BehaviorPT   = "_PT_"
	BehaviorCQ   = "_CQ_"
	BehaviorKR   = "_KR_"
	BehaviorTL   = "_TL_"
	BehaviorBL   = "_BL_"
	BehaviorGP   = "_GP_"
	BehaviorBAD  = "_BAD_"
	BehaviorHPAD = "_HPAD_"
	BehaviorMAD  = "_MAD_"
	BehaviorPF   = "_PF_"
	BehaviorPCO  = "_PCO_"
	BehaviorGNR  = "_GNR_"
	BehaviorPR   = "_PR_"
	BehaviorBA   = "_BA_"
)

var BehaviorMap = map[string]string{
	BehaviorAll:     BehaviorDataAll,
	BehaviorGroup:   BehaviorDataGroup,
	BehaviorChannel: BehaviorDataChannel,
	BehaviorVideo:   BehaviorDataVideo,
	BehaviorPhoto:   BehaviorDataPhoto,
	BehaviorVoice:   BehaviorDataVoice,
	BehaviorText:    BehaviorDataText,
	BehaviorFile:    BehaviorDataFile,
	BehaviorBot:     BehaviorDataBot,
	BehaviorLast:    BehaviorDataLast,
	BehaviorNext:    BehaviorDataNext,
}

var SearchType2Behavior = map[uint8]string{
	0: BehaviorAll,
	1: BehaviorGroup,
	2: BehaviorChannel,
	3: BehaviorVideo,
	4: BehaviorPhoto,
	5: BehaviorVoice,
	6: BehaviorText,
	7: BehaviorFile,
	8: BehaviorBot,
}

var _behaviorMap = map[string]func(*SSMRequestMsg){
	OrderStart:   handleStart,
	OrderReso:    handleReso,
	OrderDaoh:    handleDaoh,
	OrderHelp:    handleHelp,
	OrderMore:    handleMore,
	OrderPrivacy: handlePrivacy,
	OrderCReso:   handleReso,
	OrderCDaoh:   handleDaoh,
	strings.Join([]string{OrderDaoh, BehaviorAnother}, "."):          handleDaohAnother,
	strings.Join([]string{OrderDaoh, BehaviorBack}, "."):             handleDaohBack,
	strings.Join([]string{OrderHelp, BehaviorR18}, "."):              handleHelpR18,
	strings.Join([]string{OrderHelp, BehaviorFM}, "."):               handleHelpFM,
	strings.Join([]string{OrderHelp, BehaviorCL}, "."):               handleHelpCL,
	strings.Join([]string{OrderHelp, BehaviorDS}, "."):               handleHelpDS,
	strings.Join([]string{OrderHelp, BehaviorRMG}, "."):              handleHelpRMG,
	strings.Join([]string{OrderHelp, BehaviorBG}, "."):               handleHelpBG,
	strings.Join([]string{OrderHelp, BehaviorProfit}, "."):           handleHelpProfit,
	strings.Join([]string{OrderHelp, BehaviorAD}, "."):               handleHelpAD,
	strings.Join([]string{OrderHelp, BehaviorReport}, "."):           handleHelpReport,
	strings.Join([]string{OrderMore, BehaviorShowQuery}, "."):        handleMoreShowQuery,
	strings.Join([]string{OrderMore, BehaviorR18}, "."):              handleMoreR18,
	strings.Join([]string{OrderMore, BehaviorRML}, "."):              handleMoreRML,
	strings.Join([]string{OrderMore, BehaviorIMM}, "."):              handleMoreIMM,
	strings.Join([]string{OrderMore, BehaviorPAD}, "."):              handleMorePAD,
	strings.Join([]string{OrderMore, BehaviorPT}, "."):               handleMorePT,
	strings.Join([]string{OrderMore, BehaviorCQ}, "."):               handleMoreCQ,
	strings.Join([]string{OrderMore, BehaviorPT, BehaviorKR}, "."):   handleMorePADKW,
	strings.Join([]string{OrderMore, BehaviorPT, BehaviorTL}, "."):   handleMorePADTL,
	strings.Join([]string{OrderMore, BehaviorPT, BehaviorBL}, "."):   handleMorePADBL,
	strings.Join([]string{OrderMore, BehaviorPT, BehaviorGP}, "."):   handleMorePADGP,
	strings.Join([]string{OrderMore, BehaviorPT, BehaviorBAD}, "."):  handleMorePADBAD,
	strings.Join([]string{OrderMore, BehaviorPT, BehaviorHPAD}, "."): handleMorePADHPAD,
	strings.Join([]string{OrderMore, BehaviorPT, BehaviorMAD}, "."):  handleMorePADMAD,

	strings.Join([]string{OrderMore, BehaviorIMM, BehaviorPF}, "."):  handleMoreIMMPF,
	strings.Join([]string{OrderMore, BehaviorIMM, BehaviorPCO}, "."): handleMoreIMMPCO,
	strings.Join([]string{OrderMore, BehaviorIMM, BehaviorGNR}, "."): handleMoreIMMGNR,
	strings.Join([]string{OrderMore, BehaviorIMM, BehaviorPR}, "."):  handleMoreIMMPR,
	strings.Join([]string{OrderMore, BehaviorIMM, BehaviorBA}, "."):  handleMoreIMMBA,

	strings.Join([]string{OrderPrivacy, BehaviorClose}, "."): handlePrivacyClose,
}

var _resoMap = new(sync.Map)

func reso() (string, string, map[string]any, error) {
	params := make([][][]string, 0)

	if cmd := redis.Instance().ZRevRange(context.Background(), "HotRankList", 0, 39); cmd.Err() != nil {
		return "", "", map[string]any{}, cmd.Err()
	} else {

		if cmd.Val() != nil && len(cmd.Val()) > 0 {
			sub := make([][]string, 0)
			for i, v := range cmd.Val() {
				if len(sub) == 4 {
					tmp := make([][]string, len(sub))
					copy(tmp, sub)
					sub = make([][]string, 0)
					params = append(params, tmp[:])
				}

				key := fmt.Sprintf("_RESO_%02d", i)
				_resoMap.Store(key, v)

				sub = append(sub, []string{v, fmt.Sprintf("tg://resolve?domain=Pabl02025Bot&start=%s", key), ""})
			}
			if len(sub) > 0 {
				params = append(params, sub[:])
			}
		}
	}

	return "🔥近期热搜排行榜\n发送关键词🔍搜索你感兴趣的内容", ParseModeMarkdownV2, generateMarkup(params), nil
}

func daoh() (string, string, map[string]any, error) {
	return "选择你感兴趣的类别\n🔍发现更大的世界", ParseModeMarkdownV2, generateMarkup([][][]string{
		{{"♻️换一批", "", strings.Join([]string{OrderDaoh, BehaviorAnother}, ".")}},
		{{"🔍同城交友", "", strings.Join([]string{OrderDaoh, BehaviorBack}, ".")}, {"🔞成人内容", "", strings.Join([]string{OrderDaoh, BehaviorBack}, ".")}, {"🧩兴趣社区", "", strings.Join([]string{OrderDaoh, BehaviorBack}, ".")}},
		{{"🍉新闻吃瓜", "", strings.Join([]string{OrderDaoh, BehaviorBack}, ".")}, {"🎵音乐分享", "", strings.Join([]string{OrderDaoh, BehaviorBack}, ".")}, {"🎬影视资源", "", strings.Join([]string{OrderDaoh, BehaviorBack}, ".")}},
		{{"₿币圈区块链", "", strings.Join([]string{OrderDaoh, BehaviorBack}, ".")}, {"💻编程开发", "", strings.Join([]string{OrderDaoh, BehaviorBack}, ".")}, {"📌求职招聘", "", strings.Join([]string{OrderDaoh, BehaviorBack}, ".")}},
		{{"🎮游戏娱乐", "", strings.Join([]string{OrderDaoh, BehaviorBack}, ".")}, {"🌐科学上网", "", strings.Join([]string{OrderDaoh, BehaviorBack}, ".")}, {"🚀科技前沿", "", strings.Join([]string{OrderDaoh, BehaviorBack}, ".")}},
		{{"💰金融投资", "", strings.Join([]string{OrderDaoh, BehaviorBack}, ".")}, {"🌟二次元动漫", "", strings.Join([]string{OrderDaoh, BehaviorBack}, ".")}, {"📖小说阅读", "", strings.Join([]string{OrderDaoh, BehaviorBack}, ".")}},
		{{"📺主播直播", "", strings.Join([]string{OrderDaoh, BehaviorBack}, ".")}, {"🤖人工智能", "", strings.Join([]string{OrderDaoh, BehaviorBack}, ".")}, {"📰政治时事", "", strings.Join([]string{OrderDaoh, BehaviorBack}, ".")}},
		{{"🛒电商好物", "", strings.Join([]string{OrderDaoh, BehaviorBack}, ".")}, {"🧰软件工具", "", strings.Join([]string{OrderDaoh, BehaviorBack}, ".")}, {"🎓教育学习", "", strings.Join([]string{OrderDaoh, BehaviorBack}, ".")}},
		{{"🏃🏻健康运动", "", strings.Join([]string{OrderDaoh, BehaviorBack}, ".")}, {"🏖️美食旅行", "", strings.Join([]string{OrderDaoh, BehaviorBack}, ".")}, {"🎨设计创意", "", strings.Join([]string{OrderDaoh, BehaviorBack}, ".")}},
		{{"❤️情感交流", "", strings.Join([]string{OrderDaoh, BehaviorBack}, ".")}, {"📚资源分享", "", strings.Join([]string{OrderDaoh, BehaviorBack}, ".")}, {"🤖机器人", "", strings.Join([]string{OrderDaoh, BehaviorBack}, ".")}},
		{{"🔙返回", "", strings.Join([]string{OrderDaoh, BehaviorBack}, ".")}},
	}), nil
}

func help() (string, string, map[string]any, error) {
	return "请点击按钮，查看教程👇", ParseModeMarkdownV2, generateMarkup([][][]string{
		{{"🌟解决iPhone限制查看成人内容方法", "", strings.Join([]string{OrderHelp, BehaviorR18}, ".")}},
		{{"▪️寻找免费的电影资源", "https://t.me/Pabl02025Bot", ""}},
		{{"▪️下载免费的音乐并上传到播放器", "", strings.Join([]string{OrderHelp, BehaviorFM}, ".")}},
		{{"▪️把Telegram语言设置为中文", "", strings.Join([]string{OrderHelp, BehaviorCL}, ".")}},
		{{"▪️解除无法私聊限制", "https://t.me/Pabl02025Bot", ""}},
		{{"▪️Telegram防骗指南", "", strings.Join([]string{OrderHelp, BehaviorDS}, ".")}},
		{{"▪️让机器人收录我的群", "", strings.Join([]string{OrderHelp, BehaviorRMG}, ".")}},
		{{"▪️建立一个搜索群", "", strings.Join([]string{OrderHelp, BehaviorBG}, ".")}},
		{{"收益相关", "", strings.Join([]string{OrderHelp, BehaviorProfit}, ".")}, {"广告相关", "", strings.Join([]string{OrderHelp, BehaviorAD}, ".")}},
		{{"🪧广告", "https://t.me/Pabl02025Bot", ""}, {"👥交流", "https://t.me/Pabl02025Bot", ""}, {"🧭教程", "https://t.me/Pabl02025Bot", ""}},
		{{"🤝合作", "https://t.me/Pabl02025Bot", ""}, {"💢投诉", "", strings.Join([]string{OrderHelp, BehaviorReport}, ".")}},
	}), nil
}

func more() (string, string, map[string]any, error) {
	return "---------------请选择---------------", ParseModeText, generateMarkup([][][]string{
		{{"🔍热搜排行榜", "", OrderReso}, {"👥群组导航", "", OrderDaoh}},
		{{"📊曝光查询", "", strings.Join([]string{OrderMore, BehaviorShowQuery}, ".")}, {"🔞色情限制内容", "", strings.Join([]string{OrderMore, BehaviorR18}, ".")}},
		{{"💰邀请赚钱", "", strings.Join([]string{OrderMore, BehaviorIMM}, ".")}, {"🪧投放广告", "", strings.Join([]string{OrderMore, BehaviorPAD}, ".")}},
		{{"🔗收录链接", "", strings.Join([]string{OrderMore, BehaviorRML}, ".")}, {"❔帮助教程", "", OrderHelp}},
		{{"快搜互推", "https://t.me/Pabl02025Bot", ""}, {"UT钱包", "https://t.me/Pabl02025Bot", ""}, {"自动发片", "https://t.me/Pabl02025Bot", ""}},
		{{"教程", "https://t.me/Pabl02025Bot", ""}, {"公告", "https://t.me/Pabl02025Bot", ""}, {"运营", "https://t.me/Pabl02025Bot", ""}, {"客服", "https://t.me/Pabl02025Bot", ""}},
	}), nil
}

func privacy() (string, string, map[string]any, error) {
	text := `*Privacy Policy*  
*General*  
We are committed to protecting your privacy\. We strictly comply with applicable privacy laws and regulations when collecting, using, storing and protecting your data, including those of Apple and Google stores and Telegram\. In addition to the above laws and regulations, our own privacy policy is more stringent\. We strictly adhere to the principle of Occam's razor and will not collect and use any data beyond the functional purpose\.

*1\. Data Collection*  
We collect the following information:  
• User ID: A unique identifier assigned by Telegram that allows us to distinguish you from other users\. This is necessary for us to properly function and provide you with the services you request\.  
• Username and Nickname: Your Telegram username and nickname\.  
• Language: Your preferred language setting, which allows us to customize the bot's responses for you\.  
• Data you voluntarily provide: We will not and cannot collect information outside the scope of the Telegram API, so we will never collect more data from you than you provide to Telegram\. In particular, we will never and cannot collect your mobile phone number or your IP address\.

*2\. How We Use Data*  
We use your data for the following purposes:  
• Improve your experience: We use your language preference to personalize your interactions with our bot\.  
• Responding to your inquiries and requests: We use your data to respond to your inquiries and requests for support\.  
• Preventing fraud and abuse: We may use your data to prevent fraud and abuse of our services\.

*3\. Data Sharing*  
All data will be stored encrypted and will never be shared with third parties\. No third party will be involved in the safekeeping of data except the cloud service provider\. These cloud service providers are bound by confidentiality agreements and are not allowed to use your data for any other purpose except providing services to us\.  
Possible exceptions:  
We may also disclose your data if required by law or regulation, but as mentioned above, we have not collected your mobile phone number, email address, IP address and other private information, so it is impossible to disclose such pravicy information\.

*4\. Data Security*  
We take appropriate security measures to protect your personal information from unauthorized access, use, or disclosure\.

*5\. User Rights*  
You have the right to access, update, or delete your personal information\.

*6\. Policy Updates*  
We will update this Privacy Policy as laws and regulations change\. We will notify you of any significant changes\. We encourage you to review this Privacy Policy periodically to learn how we are protecting your information\.  
Send the command to the bot to get the Privacy Policy:  
\/privacy

*7\. Contact Us*  
If you have any questions or concerns about this Privacy Policy, If you have any questions about our privacy policy, please contact us at [@Pabl02025Bot](https://t.me/Pabl02025Bot)\.
`

	return text, ParseModeMarkdownV2, generateMarkup([][][]string{{{"X关闭", "", strings.Join([]string{OrderPrivacy, BehaviorClose}, ".")}}}), nil
}

func other(userID, messageID int, username, behavior, text string) (string, string, map[string]any, error) {
	st := uint8(0)
	sorts := []float64{}
	coverSort := true

	switch behavior {
	case BehaviorDataGroup:
		{
			st = 1
		}
	case BehaviorDataChannel:
		{
			st = 2
		}
	case BehaviorDataVideo:
		{
			st = 3
		}
	case BehaviorDataPhoto:
		{
			st = 4
		}
	case BehaviorDataVoice:
		{
			st = 5
		}
	case BehaviorDataText:
		{
			st = 6
		}
	case BehaviorDataFile:
		{
			st = 7
		}
	case BehaviorDataBot:
		{
			st = 8
		}
	case BehaviorDataLast:
		{
			coverSort = false
			var err error
			if sorts, st, err = getSortsAndSearchType(true, userID, messageID); err != nil {
				return "", "", map[string]any{}, err
			}
		}
	case BehaviorDataNext:
		{
			coverSort = false
			var err error
			if sorts, st, err = getSortsAndSearchType(false, userID, messageID); err != nil {
				return "", "", map[string]any{}, err
			}
		}
	}

	if coverSort {
		if err := redis.Instance().Del(context.Background(), fmt.Sprintf("Sorts:%d:%d", userID, messageID)).Err(); err != nil {
			logger.App().Errorf("delete Sorts:%d:%d error : %s", userID, messageID, err.Error())
		}
	}

	logger.App().Infof("do search by condition : [%d] [%s] %+v", st, text, sorts)

	result, err := search.Search(st, text, sorts)
	if err != nil {
		return "", "", map[string]any{}, err
	}

	logger.App().Errorf("do search success : %+v", *(result))

	value := ""
	if title, link := GetTypeAd(username, 5); title != "" && link != "" {
		value = fmt.Sprintf("广告:[%s](%s)\n\n", EscapeMarkdownV2(title), link)
	}

	if ads := GetKeywordAd(username, text); ads != nil && len(ads) != 0 {
		badges := []string{"🥇", "🥈", "🥉", "🏅"}
		for i, vs := range ads {
			if vs != nil && len(vs) >= 2 {
				k := i
				if k >= len(badges) {
					k = len(badges) - 1
				}
				value += fmt.Sprintf("%s [%s](%s)\n", badges[k], vs[0], vs[1])
			}
		}
	}

	for idx, item := range result.Content {
		value += fmt.Sprintf("%2d\\.[%s](%s)\n", idx+1, EscapeMarkdownV2(item.Content), item.Link)
	}

	params := [][][]string{
		generateSearchType(st),
		generateLastNextPage(result.Next, username, userID, messageID),
	}

	if title, link := GetTypeAd(username, 3); title != "" && link != "" {
		params = append(params, [][]string{{title, link, ""}})
	}

	go saveSortsAndSearchType(userID, messageID, result.LastSort[:], st)

	return value, ParseModeMarkdownV2, generateMarkup(params), nil
}

func generateSearchType(st uint8) [][]string {
	origin := []uint8{0, 1, 2, 3, 4, 5, 6, 7, 8}
	origin = append(origin[:int(st)], origin[int(st+1):]...)

	left := make([][]string, 0)

	for _, v := range origin {
		behavior := SearchType2Behavior[v]
		behaviorData := BehaviorMap[behavior]
		left = append(left, []string{behavior, "", behaviorData})
	}
	return left
}

func generateLastNextPage(next bool, username string, userID, messageID int) [][]string {
	sKey := fmt.Sprintf("Sorts:%d:%d", userID, messageID)

	len, err := redis.Instance().LLen(context.Background(), sKey).Result()
	if err != nil {
		logger.App().Infof("llen %s error : %s", sKey, err.Error())
		return [][]string{}
	}

	hasPrev := len >= 3

	params := [][]string{}

	if hasPrev {
		params = append(params, []string{BehaviorLast, "", BehaviorDataLast})
	} else {
		if title, link := GetTypeAd(username, 4); title != "" && link != "" {
			params = append(params, []string{title, link, ""})
		}
	}

	if next {
		params = append(params, []string{BehaviorNext, "", BehaviorDataNext})
	} else {
		if title, link := GetTypeAd(username, 4); title != "" && link != "" {
			params = append(params, []string{title, link, ""})
		}
	}

	return params
}

func generateMarkup(params [][][]string) map[string]any {
	if params == nil || len(params) == 0 {
		return map[string]any{}
	}

	buttons := make([][]map[string]any, 0)
	keys := []string{"text", "url", "callback_data"}

	for _, list := range params {
		tmp := make([]map[string]any, 0)
		for _, row := range list {
			rowTmp := make(map[string]any)
			for idx, v := range row {
				if idx >= len(keys) {
					break
				}
				rowTmp[keys[idx]] = v
			}
			tmp = append(tmp, rowTmp)
		}
		buttons = append(buttons, tmp)
	}

	return map[string]any{"inline_keyboard": buttons}
}

func getSortsAndSearchType(pop bool, userID, messageID int) ([]float64, uint8, error) {
	sKey := fmt.Sprintf("Sorts:%d:%d", userID, messageID)
	tKey := fmt.Sprintf("SearchType:%d:%d", userID, messageID)

	script := `
local latestSort = redis.call("LINDEX", KEYS[1], 0)
local tVal = redis.call("GET", KEYS[2])
`

	if pop {
		script = `

local top1 = redis.call("LPOP", KEYS[1])
local top2 = redis.call("LPOP", KEYS[1])

` + script + `

return {latestSort, tVal, top1, top2}

`
	} else {
		script = script + `

return {latestSort, tVal}

`
	}

	result, err := redis.Instance().Eval(context.Background(), script, []string{sKey, tKey}).Result()
	if err != nil {
		logger.App().Errorf("eval error : %s", err.Error())
		return []float64{}, 0, err
	}

	res := result.([]interface{})

	logger.App().Infof("================== %+v", res)

	sorts := make([]float64, 0)
	if res[0] != nil {
		sortStr := cast.ToString(res[0])
		if err = sonic.Unmarshal([]byte(sortStr), &sorts); err != nil {
			logger.App().Errorf("unmarshal error : %s", err.Error())
			return []float64{}, 0, err
		}
	}

	st := uint8(0)
	if res[1] != nil {
		st = cast.ToUint8(res[1])
	}

	return sorts, st, nil
}

func saveSortsAndSearchType(userID, messageID int, sorts []float64, st uint8) error {
	sKey := fmt.Sprintf("Sorts:%d:%d", userID, messageID)
	tKey := fmt.Sprintf("SearchType:%d:%d", userID, messageID)

	args := make([]interface{}, 0, 2)

	if data, err := sonic.Marshal(&sorts); err != nil {
		logger.App().Errorf("marshal error : %s", err.Error())
		return err
	} else {
		args = append(args, string(data))
	}
	args = append(args, strconv.Itoa(int(st)))

	luaScript := `
local sKey = KEYS[1]
local tKey = KEYS[2]
local expireSeconds = 180
local uint8Value = ARGV[#ARGV]

if redis.call("EXISTS", sKey) == 0 then
	redis.call("LPUSH", sKey, "[]")
	redis.call("LPUSH", sKey, "[]")
end

for i = 1, #ARGV - 1 do
    redis.call("LPUSH", sKey, ARGV[i])
end

redis.call("SET", tKey, uint8Value)
redis.call("EXPIRE", sKey, expireSeconds)
redis.call("EXPIRE", tKey, expireSeconds)
return true
`

	_, err := redis.Instance().Eval(context.Background(), luaScript, []string{sKey, tKey}, args...).Result()
	return err
}

func behaviorRelease18() (string, string, map[string]any, string) {

	fileID := ""
	if value, exist := _videoFileIDMap[VideoR18Desc]; exist {
		fileID = value
	}

	return `如果你进入某个群或频道遇到如下提示：
This channel can't displayed because it was used to spread pornographic content\.
原因：
有人在群/频道里发了色情内容,  被 Telegram 官方限制了;

✅解决办法：
登录Telegram Web网页版链接： https://web\.telegram\.org
（复制到浏览器打开）
⚡️操作： 登录网页版后
➊ 点击「Settings/设置」
➋ 点击「Privacy and Security/隐私和安全」
➌ 找到「Sensitive content/敏感内容」并勾选「Disable filtering/禁用过滤」
➍ 重启 iOS 客户端即可正常访问，

❓评论区问题汇总:
找不到「Disable filtering」选项；
用伊斯兰国家的电话号码注册的电报都无法禁用过滤，只能换个手机号码，从新注册尝试
登录网页版时收不到验证码；
因为你正在使用盗版的电报应用。为了避免这个问题，强烈建议你卸载非官方版本，并前往[官方网站](https://telegram.org/)下载正版电报。
打不开网页推荐这个VPN
https://imaodou\.xyz`, ParseModeMarkdownV2, generateMarkup([][][]string{}), fileID
}

func behaviorFreeMusic() (string, string, map[string]any, string) {

	fileID := ""
	if value, exist := _videoFileIDMap[VideoFMV]; exist {
		fileID = value
	}

	return "下载免费的音乐，并上传到播放器", ParseModeText, generateMarkup([][][]string{}), fileID
}

func behaviorChangeLanuage() (string, string, map[string]any, string) {

	fileID := ""
	if value, exist := _videoFileIDMap[VideoCL]; exist {
		fileID = value
	}

	return `点击链接设置 Telegram 语言为中文👇：

● [简体中文](https://t.me/setlanguage/zh-hans-beta)

● [繁体中文\(香港\)](https://t.me/setlanguage/zh-hant-beta)`, ParseModeMarkdownV2, generateMarkup([][][]string{}), fileID
}

func behaviorDefendScam() (string, string, map[string]any) {
	return `【提防诈骗】Telegram防骗指南
———————如果你在电报上收到一条陌生私信，它99%是个骗子

亲爱的用户，

在Telegram上进行交流和信息获取时，安全是首要的。我们特此编写了一份防骗指南，以帮助你避免可能的网络诈骗：

隐私设置：我们强烈建议你在Telegram的隐私设置中将手机号码设为所有人不可见，并在添加新好友时取消勾选“分享我的电话号码”，以防止个人信息泄露。

信息来源验证：请注意，有人可能会冒充你的亲友，通过仿造他们的头像和昵称来进行诈骗。在转账或分享重要信息前，务必确认对方的身份。

保护个人信息：请不要向他人透露包括密码、银行账号、身份证号等个人敏感信息。

谨慎进行虚拟货币交易：电报上的虚拟货币交易充斥着诈骗行为，尤其是买卖黑U的交易，你需要保持高度警惕。

谨慎点击文件：请谨慎对待电报上的文件，避免点击可能含有木马病毒的文件，特别是中文包和汉化电报的文件全是病毒。

避免频繁私信：过于频繁地发送私信可能会被官方视为发送垃圾信息，导致账号被强制注销。

对陌生信息保持警惕：如果你在电报上收到一条陌生人发来的信息，大概率这是诈骗行为，你要保持警惕。

确保下载正版Telegram：请务必从官方网站或认证的应用商店下载Telegram。有些第三方非官方版本可能会在你进行数字货币交易时篡改收币地址，盗取你的财产。

独立甄别信息真伪：面对海量信息，你需要学会独立思考，不轻信未经验证的信息。

在享受Telegram带来的便利的同时，也请时刻保持警惕，维护自己的网络安全。

快搜团队敬上`, ParseModeMarkdownV2, generateMarkup([][][]string{})
}

func behaviorBuildGroup() (string, string, map[string]any, string) {

	fileID := ""
	if value, exist := _videoFileIDMap[VideoBuildG]; exist {
		fileID = value
	}

	return `用快搜机器人建立自己的搜索群
第一步：建立一个公开群
第二步：邀请 @jisou 进群并将它设置为管理员
第三步：在群里弹出的快捷菜单，开启搜索
搜索群示例：@jisou0`, ParseModeMarkdownV2, generateMarkup([][][]string{}), fileID
}

func behaviorRecordMyGroup() (string, string, map[string]any) {
	return `向机器人发送群的链接，它会自动收录
或者把机器人邀请进群或频道，也能自动收录`, ParseModeMarkdownV2, generateMarkup([][][]string{})
}

func behaviorProfit() (string, string, map[string]any) {
	return EscapeMarkdownV2(`▪️直推用户搜索收益归属？
搜索群进行搜索的收益归为群的受益人，私聊机器人搜索收益归上级受益人，采用邀请链接和群推广快搜都可以成为用户的上级

▪️为什么我的绑定的用户变少了？
把群解散和把机器人踢出群，会丢失通过这个群绑定的直推和裂变用户，把机器人邀请进群又会恢复绑定。把群解散和把机器人踢出群，会丢失通过这个群绑定的直推和裂变用户，把机器人邀请进群又会恢复绑定。同理你的下级用户将机器人踢出群，随之你的裂变也会随之减少

▪️为什么有些搜索次数很多，但是收益确不多？
单个用户每天的只要前20次搜索为有效搜索，超过20次之后无效，没有收益

▪️搜索群有很多人搜索，但是为什么没有收益？
检查是否开启搜索分红

▪️用户和受益人是怎么形成绑定的？
在搜索群搜一次、或通过邀请链接绑定

▪️群的受益人是怎么立的？
谁升级为机器人为管理员谁就是受益人

▪️收益分几部分组成？
拉新奖励、搜索收益、群置顶收益

▪️拉新奖励？
用邀请链接直接推广一个用户，获得0.08$。邀请机器人进群组|频道，使用快搜自带的分享功能，推广一个新用户获得0.16$。裂变一个用户获得0.02$

▪️搜索收益？
直推用户的一次有效搜索0.0036$，裂变用户的一次有效搜索0.0009$

▪️群置顶收益？
基于群的活跃度，发放收益

▪️提现多久到账？
1~3个工作日到账

▪️有哪些情况提现会被驳回？
拉新数据异常，提现被驳回。等7天重新发起提现

▪️收益被清空？
收益被清空就是命中作弊，如果没有作弊，请联系管理员申诉`), ParseModeMarkdownV2, generateMarkup([][][]string{{{"🔙返回", "", OrderHelp}}})
}

func behaviorAD() (string, string, map[string]any) {
	return `▪️统计广告效果
在频道或群新建一个邀请链接，将这个链接设置成广告链接就能统计出从这个邀请链接进入了多少人，从而大致统计出广告效果。

▪️充值到账时间
充值后通常在5分钟内到账，我们支持USDT和支付宝作为支付方式。

▪️禁入广告类型
1、竞争对手产品：不接受与快搜竞争的任何产品广告。
2、不得露点：视频或图片中不得出现露点或过度暴露的内容。
3、严禁以下内容：
枪支：不得宣传或销售武器。
毒品：禁止任何非法药物的广告。
诈骗：不接受任何形式的欺诈或骗局。
涉幼：禁止与未成年有不当行为的内容。
恐怖主义：禁止宣传恐怖主义或支持恐怖活动。
涉政：禁止与政治相关或有争议的广告。

▪️查看广告展现位置
私聊机器人输入： /adshow\[空格\] 广告链接，广告主可以查看自己的广告在公开群的展示位置

▪️关键词展现规则
采用模糊匹配的规则
购买了关键词“北京”的广告，包含“北京”的搜索词如“北京同城”会触发你的广告显示。但如果“北京同城”已被另一个广告单独购买，则不会显示你的“北京”广告。

▪️关键词定价规则
关键词排名广告收费策略有两种：直接购买和竞拍获得。尚未售出的关键词可以直接购买。已经售出的关键词则需要通过竞价购买（意向关键词可以设置竞拍提醒）

▪️关键词续费与竞拍规则：
广告主在广告到期前23天，可支付原价加20%直接续费，避免竞拍。
若未续费，关键词将进入竞拍，以上次成交价为底价。
竞拍中，每次加价必须是当前价的10%。
拍卖结束前10分钟有新出价，拍卖自动延长10分钟。
若竞拍无人出价，原广告主可按原价续费

▪️顶部链接和按钮广告展现规则
采用均衡展现策略，确保在整月内平均展示。若月初两天的展现次数超标，第三天系统会自动调整减少，避免月底展现骤减。这确保了广告在全月都得到恰当的关注。


▪️置顶广告展现规则
置顶广告每半小时在活跃的搜索群中轮换一次。当下一个广告被置顶时，前一个置顶广告会被删除。

▪️品牌广告展现规则
当用户输入与您品牌相关的关键词时（可设5个关键词），他们首先会看到与您品牌相关的专属搜索结果。`, ParseModeMarkdownV2, generateMarkup([][][]string{{{"🔙返回", "", OrderHelp}}})
}

func behaviorReport() (string, string, map[string]any) {
	return `涉嫌以下行为的群/频道都被拉黑：传播未成年色情视频、宣扬恐怖主义、涉嫌毒品、枪支、诈骗。（投诉诈骗必需提供聊天截图、支付记录证据，否则不受理）

请按以下格式发送给客服否则不予受理

投诉对象：
群/频道：
原因：
证据：`, ParseModeMarkdownV2, generateMarkup([][][]string{{{"☎️联系客服", "https://t.me/JISOUKFbot", ""}}, {{"🔙返回", "", OrderHelp}}})
}

func behaviorShowQuery() (string, string, map[string]any) {
	return `输入：“/url\[空格\] \[链接\]”，查询机器人给该链接的曝光次数

输入：“/adshow\[空格\] \[广告链接或ID\]”，查询广告展现位置`, ParseModeMarkdownV2, generateMarkup([][][]string{})
}

func behaviorRecordMyLink() (string, string, map[string]any) {
	return `[收录链接，请邀请快搜加入并提升为管理员。](https://t.me/jisou123bot?startgroup=true)💡输入“/url\[空格\]\[链接\]，可以查询该链接在快搜的曝光次数。

你的链接：

[🤖 快搜互推](https://t.me/hutui1bot) \| [🤖自动发片](https://t.me/ziyuan1bot) \| [📜收录指南](https://t.me/jisou1/11)`, ParseModeMarkdownV2, generateMarkup([][][]string{{{"+收录链接请邀请快搜加入", "https://t.me/Pabl02025Bot?startgroup=true", ""}}})
}

func behaviorInviteMakeMoney(userID int, flName string) (string, string, map[string]any) {
	return fmt.Sprintf(`邀请好友使用快搜，您就能持续从好友的搜索中获得收益。💰收益分为两部分组成，拉新奖励和搜索收益

拉新奖励：
邀请方式一：使用邀请链接直推一个新用户获得0\.08$拉新奖励
邀请方式二：把快搜机器人邀请进群组\|频道，就会收到一条推送，然后点击在“此群组\|频道分享快搜”。机器人就会定时向此群组\|频道推送拉新文案。新用户点此文案下方按钮进入快搜，你会获得0\.16$拉新奖励

裂变奖励：
你的一级直推，每裂变一个用户，你会获得0\.02$二级裂变奖励

搜索收益：
你的直推用户每进行一次搜索，你会获得0\.0036$收益。你的二级裂变用户每进行一次搜索你获得0\.0009$收益。

创建搜索群：
创建一个搜索群，将快搜邀请进群，并开启搜索。只要有人在你的群每进行一次搜索你都会获得0\.0036$收益。开启了置顶权限且群日活超过30人，还会有置顶广告收益。

⚠注意！通过邀请方式一和邀请方式二拉新，您才能获得拉新奖励。但搜索收益无论是在群聊中或私聊机器人进行搜索，您都将长期获得搜索收益。

单击复制专属分享链接：
🔍快搜Telegram必备的搜索引擎，帮你轻松找到想要的群组、频道、视频、音乐👉 t\.me/Pabl02025Bot?start\=a\_%d

收益账户：
👤%s\(%d\)
已提现收益：0$
待入账收益：0$
可提现收益：0$`, userID, flName, userID), ParseModeMarkdownV2, generateMarkup([][][]string{
			{{"📩获取推广参考文案", "", strings.Join([]string{OrderMore, BehaviorPT}, ".")}},
			{{"➕邀请进群", "https://t.me/Pabl02025Bot?startgroup=true", ""}},
			{{"📈推广报表", "", strings.Join([]string{OrderMore, BehaviorIMM, BehaviorPF}, ".")}, {"💵收益体现", "", strings.Join([]string{OrderMore, BehaviorIMM, BehaviorPCO}, ".")}},
			{{"🏆拉新排行榜", "", strings.Join([]string{OrderMore, BehaviorIMM, BehaviorGNR}, ".")}, {"💰收益排行榜", "", strings.Join([]string{OrderMore, BehaviorIMM, BehaviorPR}, ".")}},
			{{"🕴️广告代理", "", strings.Join([]string{OrderMore, BehaviorIMM, BehaviorBA}, ".")}, {"⁉️常见问题", "", strings.Join([]string{OrderMore, BehaviorCQ}, ".")}},
			{{"💬官方交流群", "https://t.me/duibai0", ""}},
			{{"<返回", "", OrderMore}},
		})
}

func behaviorPutAD() (string, string, map[string]any) {
	return `⽤快搜建⽴的搜索群，累计76340个。覆盖⽤⼾9515万人
快搜加入的频道，累计48368个。覆盖⽤⼾33789万人

[【色搜版】人人都是鉴黄师](https://t.me/selaosiji) \- 200k
[搜群神器\|中文频道\|中文导航群](https://t.me/sobaidu) \- 200k
[中文搜索\|超级搜索\|中文导航群️️](https://t.me/zwdhqun1) \- 200k
[中文群组\|搜索引擎\|中文搜索群](https://t.me/zwdhqun) \- 197k
[中文搜索\|中文导航\|搜群神器\|中文群组\|中文频道](https://t.me/zwss188) \- 194k
[中文搜索 🔍中文导航\|快搜搜索\|超级搜索\|超级索引](https://t.me/TGDH5) \- 194k
[Telegram 中文社群](https://t.me/zwss1234) \- 186k
[提速搜](https://t.me/pkcbb) \- 185k
[TG\-全能搜索🔍](https://t.me/+1c6JVkdC8IgzNGVl) \- 183k
[吃瓜搜片小能手🍉](https://t.me/+HzC49-whIq1jNjc1) \- 183k
[搜群神器\|中文搜索\|中文导航群](https://t.me/hao1234bot_superindexcnbot) \- 158k
[🔸🔹老色批万能搜索站🔸🔹](https://t.me/+xrbzIl5N4YsxZTQ1) \- 151k
[【备用】电报搜索全能王](https://t.me/baidu55a) \- 150k
[TG资源极速🔍搜索](https://t.me/tgsou0) \- 139k
[萝莉学生评论区](https://t.me/+XHpKec_GJYwzNDMx) \- 138k
[影视搜索](https://t.me/sousuozhan) \- 138k
[中文群组/搜索引擎/中文导航](https://t.me/soso_su7) \- 135k
[吃瓜ღ大赛](https://t.me/jiushichigua) \- 129k
[中文搜索\|中文导航\|搜索引擎\|超级搜索](https://t.me/sousuoyinqing_888) \- 125k
[最新🫤吃瓜（独立广告）](https://t.me/vgcgsb) \- 118k

快搜提供5种⼴告投放形式：关键词排名、顶部链接、底部按钮、群置顶、品牌广告。点击下方按钮进行投放。`, ParseModeMarkdownV2, generateMarkup([][][]string{
			{{"🥇关键词排名", "", strings.Join([]string{OrderMore, BehaviorPT, BehaviorKR}, ".")}},
			{{"🌐顶部链接", "", strings.Join([]string{OrderMore, BehaviorPT, BehaviorTL}, ".")}},
			{{"🌐底部按钮", "", strings.Join([]string{OrderMore, BehaviorPT, BehaviorBL}, ".")}},
			{{"🛸群置顶", "", strings.Join([]string{OrderMore, BehaviorPT, BehaviorGP}, ".")}},
			{{"💫品牌广告", "", strings.Join([]string{OrderMore, BehaviorPT, BehaviorBAD}, ".")}},
			{{"🚀互推广告", "", strings.Join([]string{OrderMore, BehaviorPT, BehaviorHPAD}, ".")}},
			{{"👳🏻个人广告中心", "", strings.Join([]string{OrderMore, BehaviorPT, BehaviorMAD}, ".")}},
		})
}

func behaviorPT(userID int) (string, string, map[string]any) {
	return fmt.Sprintf(`TG必备的搜索引擎，极搜[JISOU](http://t.me/jisou?start=a_%d)帮你精准找到，想要的群组、频道、音乐 、视频

👉 t\.me/jisou?start\=a\_%d`, userID, userID), ParseModeMarkdownV2, generateMarkup([][][]string{{{"🔍资源搜索", fmt.Sprintf("http://t.me/Pabl02025Bot?start=a_%d", userID), ""}}})
}

func behaviorCQ() (string, string, map[string]any) {
	return EscapeMarkdownV2(`▪️直推用户搜索收益归属？
搜索群进行搜索的收益归为群的受益人，私聊机器人搜索收益归上级受益人，采用邀请链接和群推广极搜都可以成为用户的上级

▪️为什么我的绑定的用户变少了？
把群解散和把机器人踢出群，会丢失通过这个群绑定的直推和裂变用户，把机器人邀请进群又会恢复绑定。把群解散和把机器人踢出群，会丢失通过这个群绑定的直推和裂变用户，把机器人邀请进群又会恢复绑定。同理你的下级用户将机器人踢出群，随之你的裂变也会随之减少

▪️为什么有些搜索次数很多，但是收益确不多？
单个用户每天的只要前20次搜索为有效搜索，超过20次之后无效，没有收益

▪️搜索群有很多人搜索，但是为什么没有收益？
检查是否开启搜索分红

▪️用户和受益人是怎么形成绑定的？
在搜索群搜一次、或通过邀请链接绑定

▪️群的受益人是怎么立的？
谁升级为机器人为管理员谁就是受益人

▪️收益分几部分组成？
拉新奖励、搜索收益、群置顶收益

▪️拉新奖励？
用邀请链接直接推广一个用户，获得0.08$。邀请机器人进群组|频道，使用极搜自带的分享功能，推广一个新用户获得0.16$。裂变一个用户获得0.02$

▪️搜索收益？
直推用户的一次有效搜索0.0036$，裂变用户的一次有效搜索0.0009$

▪️群置顶收益？
基于群的活跃度，发放收益

▪️提现多久到账？
1~3个工作日到账

▪️有哪些情况提现会被驳回？
拉新数据异常，提现被驳回。等7天重新发起提现

▪️收益被清空？
收益被清空就是命中作弊，如果没有作弊，请联系管理员申诉`), ParseModeMarkdownV2, generateMarkup([][][]string{{{"<返回", "", strings.Join([]string{OrderMore, BehaviorIMM}, ".")}}})
}

func behaviorPADKW() (string, string, map[string]any) {
	return "👉 发送 \"/kw 关键词\" 来查询关键词价格，例如发送：/kw 深圳", ParseModeMarkdownV2, generateMarkup([][][]string{
		{{"<返回", "", strings.Join([]string{OrderMore, BehaviorPAD}, ".")}, {"热搜词", "", strings.Join([]string{OrderMore, BehaviorPAD}, ".")}},
	})
}

func behaviorPADTL() (string, string, map[string]any) {
	return `📢 顶部链接
此广告将会展示在搜索结果的顶部，并在一个月内不同时段均匀展示。
可选套餐如下：`, ParseModeMarkdownV2, generateMarkup([][][]string{
			{{">30万次展现/月=500$", "", strings.Join([]string{OrderMore, BehaviorPAD}, ".")}},
			{{">60万次展现/月=910$", "", strings.Join([]string{OrderMore, BehaviorPAD}, ".")}},
			{{">120万次展现/月=1680$", "", strings.Join([]string{OrderMore, BehaviorPAD}, ".")}},
			{{">240万次展现/月=3220$", "", strings.Join([]string{OrderMore, BehaviorPAD}, ".")}},
			{{">480万次展现/月=6160$", "", strings.Join([]string{OrderMore, BehaviorPAD}, ".")}},
			{{">960万次展现/月=12000$", "", strings.Join([]string{OrderMore, BehaviorPAD}, ".")}},
			{{"<返回", "", strings.Join([]string{OrderMore, BehaviorPAD}, ".")}},
		})
}

func behaviorPADBL() (string, string, map[string]any) {
	return `📢 底部按钮
此广告将会展示在搜索结果的底部按钮，并在一整天中不同时段均匀展示。
可选套餐如下：`, ParseModeMarkdownV2, generateMarkup([][][]string{
			{{">30万次展现/月=450$", "", strings.Join([]string{OrderMore, BehaviorPAD}, ".")}},
			{{">60万次展现/月=850$", "", strings.Join([]string{OrderMore, BehaviorPAD}, ".")}},
			{{">120万次展现/月=1600$", "", strings.Join([]string{OrderMore, BehaviorPAD}, ".")}},
			{{">240万次展现/月=3100$", "", strings.Join([]string{OrderMore, BehaviorPAD}, ".")}},
			{{">480万次展现/月=6000$", "", strings.Join([]string{OrderMore, BehaviorPAD}, ".")}},
			{{">960万次展现/月=11800$", "", strings.Join([]string{OrderMore, BehaviorPAD}, ".")}},
			{{"<返回", "", strings.Join([]string{OrderMore, BehaviorPAD}, ".")}},
		})
}

func behaviorPADGP() (string, string, map[string]any) {
	return `📢 群轮播置顶
广告说明：置顶广告会在活跃人数50个以上的所有公开搜索群轮流置顶，每次置顶持续30分钟。您可以自由设置图片、视频和按钮内容。
轮播搜索群数量：1544个
覆盖用户数：16034474人
当前轮播数量：260个置顶广告正在轮播。

👇选择更多的轮播位将增加您广告的置顶次数`, ParseModeMarkdownV2, generateMarkup([][][]string{
			{{"1个轮播位=450$/月", "", strings.Join([]string{OrderMore, BehaviorPAD}, ".")}},
			{{"2个轮播位=900$/月", "", strings.Join([]string{OrderMore, BehaviorPAD}, ".")}},
			{{"4个轮播位=1700$/月", "", strings.Join([]string{OrderMore, BehaviorPAD}, ".")}},
			{{"8个轮播位=3300$/月", "", strings.Join([]string{OrderMore, BehaviorPAD}, ".")}},
			{{"16个轮播位=6500$/月", "", strings.Join([]string{OrderMore, BehaviorPAD}, ".")}},
			{{"<返回", "", strings.Join([]string{OrderMore, BehaviorPAD}, ".")}},
		})
}

func behaviorPADBAD() (string, string, map[string]any) {
	return `❇️品牌广告
说明：当用户输入与您品牌相关的关键词时，他们首先会看到与您品牌相关的专属搜索结果。品牌提供独特的展现机会，从而提升品牌形象。
资格限制：
· 只有具有一定知名度的品牌才能购买此功能，如：“币安”和“欧意”。
· 被视为通用词汇或非品牌专有的词汇不能作为独家关键词来购买，例如“外围”和“数据”。
· 审核未通过的品牌将全额退款。
· 任何欺诈都会被下架广告，不允退款。
请选择您想购买的时长👇👇👇`, ParseModeMarkdownV2, generateMarkup([][][]string{
			{{"三个月1000$", "", strings.Join([]string{OrderMore, BehaviorPAD}, ".")}},
			{{"六个月1600$", "", strings.Join([]string{OrderMore, BehaviorPAD}, ".")}},
			{{"一年3000$", "", strings.Join([]string{OrderMore, BehaviorPAD}, ".")}},
			{{"<返回", "", strings.Join([]string{OrderMore, BehaviorPAD}, ".")}},
		})
}

func behaviorPADHPAD() (string, string, map[string]any) {
	return `暂未开放`, ParseModeMarkdownV2, generateMarkup([][][]string{
		{{"<返回", "", strings.Join([]string{OrderMore, BehaviorPAD}, ".")}},
	})
}

func behaviorPADMAD(userID int, username, plName string) (string, string, map[string]any) {
	return fmt.Sprintf(`📈极搜广告中心

昵称：[%s](https://t.me/%s)
ID：[%d](https://t.me/%s)
💵余额：0$`, plName, username, userID, username), ParseModeMarkdownV2, generateMarkup([][][]string{
			{{"👳🏻我的广告", "", strings.Join([]string{OrderMore, BehaviorPAD}, ".")}},
			{{"🧾历史账单", "", strings.Join([]string{OrderMore, BehaviorPAD}, ".")}, {"💰充值", "", strings.Join([]string{OrderMore, BehaviorPAD}, ".")}},
			{{"🎊优惠活动", "", strings.Join([]string{OrderMore, BehaviorPAD}, ".")}, {"👩联系客服", "https://t.me/duibai0", ""}, {"❓常见问题", "", strings.Join([]string{OrderMore, BehaviorPAD}, ".")}},
			{{"<返回", "", strings.Join([]string{OrderMore, BehaviorPAD}, ".")}},
		})
}

func behaviorIMMPF() (string, string, map[string]any) {
	return `累计直推：0
累计裂变：0`, ParseModeMarkdownV2, generateMarkup([][][]string{
			{{"🧾佣金账单", "", strings.Join([]string{OrderMore, BehaviorIMM}, ".")}, {"🧑‍🤝‍🧑下级用户", "", strings.Join([]string{OrderMore, BehaviorIMM}, ".")}},
			{{"<返回", "", strings.Join([]string{OrderMore, BehaviorIMM}, ".")}},
		})
}

func behaviorIMMPCO() (string, string, map[string]any) {
	return `请选择`, ParseModeMarkdownV2, generateMarkup([][][]string{
		{{"➡️立即提现", "", strings.Join([]string{OrderMore, BehaviorIMM}, ".")}},
		{{"📝提现记录", "", strings.Join([]string{OrderMore, BehaviorIMM}, ".")}, {"🔂划转到广告账户", "", strings.Join([]string{OrderMore, BehaviorIMM}, ".")}},
		{{"<返回", "", strings.Join([]string{OrderMore, BehaviorIMM}, ".")}},
	})
}

func behaviorIMMGNR() (string, string, map[string]any) {
	return EscapeMarkdownV2(`🎉今日拉新排行榜🎉
🥇ky. - 2601人
🥈风云 2017年 - 1163人
🥉zzz - 437人
———————————————
🎖莜莜 - 370人
🎖心醉【認准地址-10 - 290人
🎖Yoki - 262人
🎖空谷 - 249人
🎖NZTG - 243人
🎖吹风吹 - 222人
🎖Joshua |🐍🌱SEED🐾 - 205人
🎖006 - 196人
🎖相顾 - 164人
🎖极搜 - 160人
🎖售后处理 - 141人
🎖bojia - 134人
🎖中文搜索 - 131人
🎖A Hsiang(商务窗口) - 122人
🎖推王 - 111人
🎖KaLang - 106人
🎖001 - 104人`), ParseModeMarkdownV2, generateMarkup([][][]string{
			{{"📨官方交流群", "https://t.me/duibai0", ""}},
		})
}

func behaviorIMMPR() (string, string, map[string]any) {
	return EscapeMarkdownV2(`💰2025-07-13收益排行榜💰

🥇极搜广告招商拾伍 - 503.02$
🥈鉴黄の小新 - 326.81$
🥉ky. - 300.16$
———————————————
🎖TANG - 285.6$
🎖樱桃🍒 - 274.71$
🎖专注🧘当下 - 250.33$
🎖西门吹牛 - 219.14$
🎖Luna - 213.69$
🎖赢了呀 - 196.53$
🎖极搜官方商务-乐乐 - 194.93$
🎖TGNZ - 175.41$
🎖@Telegram - 162.87$
🎖中文搜索 - 156.17$
🎖永旺抽奖号 不做任 - 150.15$
🎖李鬼 - 145.5$
🎖勿扰！ - 143.72$
🎖小灵通 @gg10010 - 141.59$
🎖叶 - 119.5$
🎖圣人 - 117.45$
🎖推王 - 116.36$

当日发放收益总和：30657.99$
参与分红人数总和：18751人
(数据每天凌晨0点30分更新)`), ParseModeMarkdownV2, generateMarkup([][][]string{
			{{"📨官方交流群", "https://t.me/duibai0", ""}},
		})
}

func behaviorIMMBA() (string, string, map[string]any) {
	return EscapeMarkdownV2(`您名下用户搜索超过10万次才能成为广告代理，你推广的用户已经搜索74次，继续努力吧`), ParseModeMarkdownV2, generateMarkup([][][]string{})
}
