package core

import (
	"fmt"
	"jarvis/dao/db/mysql"
	"jarvis/logger"
	"jarvis/middleware/mq/nats"
	"operate-backend/core/structure"
	"sync"
	"time"

	"gorm.io/gorm"
)

var (
	_kLocker   = new(sync.RWMutex)
	_aLocker   = new(sync.RWMutex)
	_kaLocker  = new(sync.RWMutex)
	_kMap      = map[uint]*structure.Keyword{}
	_wMap      = map[string]uint{}
	_aMap      = map[uint]*structure.Ad{}
	_kaMap     = map[uint][]uint{}
	_taLocker  = new(sync.RWMutex)
	_tADMap    = map[uint8][]uint{} // 2-置顶广告 3-搜索內连大广告 4-搜索內连小广告 5-搜索内容广告
	_tADIdxMap = map[uint8]uint64{}
)

func loadKeywordCache() error {
	if err := loadKeyword(); err != nil {
		return err
	}

	if err := loadAd(); err != nil {
		return err
	}

	if err := loadKeywordAd(); err != nil {
		return err
	}

	return nil
}

func loadKeyword() error {
	tmp := make([]*structure.Keyword, 0)
	if err := mysql.Instance().Model(new(structure.Keyword)).Find(&tmp).Error; err != nil {
		return err
	}
	m := make(map[uint]*structure.Keyword)
	wm := make(map[string]uint)
	for _, item := range tmp {
		m[item.ID] = item
		wm[item.Word] = item.ID
	}

	_kLocker.Lock()
	_kMap = m
	_wMap = wm
	_kLocker.Unlock()

	return nil
}

func loadAd() error {
	tmp := make([]*structure.Ad, 0)
	if err := mysql.Instance().Model(new(structure.Ad)).Find(&tmp).Error; err != nil {
		return err
	}
	m := make(map[uint]*structure.Ad)
	tADMap := make(map[uint8][]uint)
	tADIdxMap := make(map[uint8]uint64)
	for _, item := range tmp {
		m[item.ID] = item
		if item.Type != 1 {
			list, exist := tADMap[item.Type]
			if !exist {
				list = make([]uint, 0)
				tADIdxMap[item.Type] = 0
			}
			list = append(list, item.ID)
			tADMap[item.Type] = list
		}
	}

	_aLocker.Lock()
	_aMap = m
	_aLocker.Unlock()

	_taLocker.Lock()
	_tADMap = tADMap
	_tADIdxMap = tADIdxMap
	_taLocker.Unlock()

	return nil
}

func loadKeywordAd() error {
	tmp := make([]*structure.KeywordAd, 0)
	if err := mysql.Instance().Model(new(structure.KeywordAd)).Find(&tmp).Error; err != nil {
		return err
	}
	m := make(map[uint][]uint)
	for _, item := range tmp {
		list, exist := m[uint(item.KeywordID)]
		if !exist {
			list = make([]uint, 0)
		}
		list = append(list, uint(item.AdID))
		m[uint(item.KeywordID)] = list
	}

	_kaLocker.Lock()
	_kaMap = m
	_kaLocker.Unlock()
	return nil
}

func GetKeywordAd(username, word string) [][]string {
	if word == "" {
		return [][]string{}
	}

	_kLocker.RLock()
	defer _kLocker.RUnlock()

	theKWID, exist := _wMap[word]
	if !exist {
		return [][]string{}
	}

	keyword, kE := _kMap[theKWID]
	if !kE {
		return [][]string{}
	}

	if keyword.Status != 1 {
		return [][]string{}
	}

	_kaLocker.RLock()
	defer _kaLocker.RUnlock()

	list, adsE := _kaMap[theKWID]
	if !adsE {
		return [][]string{}
	}

	_aLocker.RLock()
	defer _aLocker.RUnlock()

	now := time.Now()

	result := make([][]string, 0)

	for _, adID := range list {
		ad, adE := _aMap[adID]
		if !adE {
			continue
		}

		if ad.Status != 1 || ad.ClientID == 0 || ad.Impressions >= ad.MaxImpressions || now.Before(time.Unix(int64(ad.StartTime), 0)) || now.After(time.Unix(int64(ad.StopTime), 0)) {
			continue
		}

		go notifyImpressions(ad.ID)

		go doCalculate(username, ad.ID, uint(ad.ClientID), ad.PricePerView)

		result = append(result, []string{ad.Title, ad.Link})
	}

	return result
}

func GetTypeAd(username string, t uint8) (string, string) {
	// 2-置顶广告 3-搜索內连大广告 4-搜索內连小广告 5-搜索内容广告
	if t < 2 || t > 5 {
		return "", ""
	}

	_taLocker.RLock()
	defer _taLocker.RUnlock()

	list, exist := _tADMap[t]
	if !exist || list == nil || len(list) == 0 {
		return "", ""
	}

	idx, idxE := _tADIdxMap[t]
	if !idxE {
		logger.App().Warnf("type %d ad have list but no index", t)
		return "", ""
	}
	_tADIdxMap[t] = idx + 1

	index := idx % uint64(len(list))

	theADID := list[index]

	_aLocker.RLock()
	defer _aLocker.RUnlock()
	ad, adE := _aMap[theADID]
	if !adE {
		logger.App().Warnf("type %d ad id not exist", theADID)
		return "", ""
	}

	now := time.Now()

	if ad.Status != 1 || ad.ClientID == 0 || ad.Impressions >= ad.MaxImpressions || now.Before(time.Unix(int64(ad.StartTime), 0)) || now.After(time.Unix(int64(ad.StopTime), 0)) {
		return "", ""
	}

	go notifyImpressions(ad.ID)

	go doCalculate(username, ad.ID, uint(ad.ClientID), ad.PricePerView)

	return ad.Title, ad.Link
}

func notifyImpressions(aid uint) {
	if err := mysql.Instance().Model(new(structure.Ad)).Where("id = ?", aid).UpdateColumns(map[string]any{
		"impressions": gorm.Expr("impressions + ?", 1),
	}).Error; err != nil {
		logger.App().Errorf("update %d impressions error : %s", aid, err.Error())
	}

	if err := nats.Instance().Publish(JSSearchImpSubject, []byte(fmt.Sprintf("%d", aid))); err != nil {
		logger.App().Errorf("publish %d impressions error : %s", aid, err.Error())
	}
}

func doCalculate(username string, aid, cid uint, price float64) {
	adLog := &structure.ADLog{AdID: uint64(aid), Username: username, Price: price}
	if err := mysql.Instance().Model(adLog).Create(adLog).Error; err != nil {
		logger.App().Errorf("do calculate %d-%s error : %s", cid, username, err.Error())
		return
	}

	if err := mysql.Instance().Model(new(structure.Client)).Where("id = ?", cid).UpdateColumns(map[string]any{
		"balance": gorm.Expr("balance - ?", price),
		"spent":   gorm.Expr("spent + ?", price),
	}).Error; err != nil {
		logger.App().Errorf("do calculate %d-%f error : %s", cid, price, err.Error())
	}
}

func doADImpressions(aid uint) {
	_aLocker.Lock()
	defer _aLocker.Unlock()

	ad, exist := _aMap[aid]
	if exist {
		ad.Impressions += 1
	}
}
