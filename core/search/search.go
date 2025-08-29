package search

import (
	"jarvis/logger"
	"sync"
)

func Init(index string, amount uint64) error {
	logger.App().Infoln("=================================================== start init ===================================================")
	defer logger.App().Infoln("=================================================== stop init ===================================================")

	_pm = &pitManager{
		index:   "message",
		amount:  2,
		close:   make(chan struct{}),
		done:    make(chan struct{}),
		mutex:   new(sync.Mutex),
		last:    make([]string, 0),
		current: make([]string, 0),
		next:    make([]string, 0),
		idx:     0,
	}

	return nil
}

func Start() error {
	go _pm.Start()

	return nil
}

func Shutdown() error {
	logger.App().Infoln("=================================================== start shutdown search ===================================================")
	defer logger.App().Infoln("=================================================== stop shutdown search ===================================================")

	_pm.Shutdown()

	return nil
}

func LoadCache() error {
	logger.App().Infoln("=================================================== start load search cache ===================================================")
	defer logger.App().Infoln("=================================================== stop load search cache ===================================================")

	return nil
}
