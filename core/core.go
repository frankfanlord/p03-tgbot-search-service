package core

import (
	"context"
	"errors"
	"jarvis/logger"
	"jarvis/middleware/mq/nats"
	"net/http"
	"time"

	"search-service/core/search"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

var (
	_server  *http.Server
	_close   = make(chan struct{})
	_done    = make(chan struct{})
	_channel = make(chan SSMRequestMsg, 10000)
	_check   = make(chan SSMRequestMsg, 10000)
)

func Init(prefix, addr string) error {
	logger.App().Infoln("=================================================== start init core ===================================================")
	defer logger.App().Infoln("=================================================== stop init core ===================================================")

	// ============= es mapping

	if err := initESMapping(); err != nil {
		return err
	}

	// ============= search

	if err := search.Init("message", 10); err != nil {
		return err
	}

	go parseAndStore()
	go flushRankList()
	go checkAndPin()
	// ============= web test

	gin.DefaultWriter = logger.GinWriter(logrus.Fields{"component": "search-web"})
	gin.SetMode(gin.DebugMode)

	engine := gin.New()
	engine.Use(gin.Recovery(), gin.Logger())

	grouper := engine.Group(prefix)

	grouper.POST("/search", doSearch)

	_server = &http.Server{Addr: addr, Handler: engine}

	// ============= nats
	if subscription, err := nats.Instance().Subscribe(JSSearchCacheSubject, doCache); err != nil {
		return err
	} else {
		_subscriptions = append(_subscriptions, subscription)
		logger.App().Infof("=========== subscribe to [%s] success ===========", JSSearchCacheSubject)
	}

	if subscription, err := nats.Instance().Subscribe(JSSearchImpSubject, doImpressions); err != nil {
		return err
	} else {
		_subscriptions = append(_subscriptions, subscription)
		logger.App().Infof("=========== subscribe to [%s] success ===========", JSSearchImpSubject)
	}

	if subscription, err := nats.Instance().QueueSubscribe(SSMissionRequestSubject, SSQueue, doRequest); err != nil {
		return err
	} else {
		_subscriptions = append(_subscriptions, subscription)
		logger.App().Infof("=========== subscribe to [%s]-[%s] success ===========", SSMissionRequestSubject, SSQueue)
	}

	return nil
}

func Start() error {
	logger.App().Infoln("=================================================== start core ===================================================")
	defer logger.App().Infoln("=================================================== stop core ===================================================")

	if err := search.Start(); err != nil {
		return err
	}

	if err := _server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.App().Errorf("failed to start server: %s", err.Error())
	}

	return nil
}

func Shutdown() error {
	logger.App().Infoln("=================================================== start shutdown core ===================================================")
	defer logger.App().Infoln("=================================================== stop shutdown core ===================================================")

	if _subscriptions != nil && len(_subscriptions) > 0 {
		for idx := range _subscriptions {
			subscription := _subscriptions[idx]
			_ = subscription.Unsubscribe()
		}
	}

	if err := search.Shutdown(); err != nil {
		return err
	}

	close(_close)

	<-_done

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(5))
	defer cancel()
	return _server.Shutdown(ctx)
}

func LoadCache() error {
	logger.App().Infoln("=================================================== start load core cache ===================================================")
	defer logger.App().Infoln("=================================================== stop load core cache ===================================================")

	if err := loadVideMapCache(); err != nil {
		return err
	}

	if err := search.LoadCache(); err != nil {
		return err
	}

	if err := loadKeywordCache(); err != nil {
		return err
	}

	return nil
}
