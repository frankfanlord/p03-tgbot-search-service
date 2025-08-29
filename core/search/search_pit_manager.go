package search

import (
	"encoding/json"
	"errors"
	"fmt"
	"jarvis/dao/db/elasticsearch"
	"jarvis/logger"
	"strings"
	"sync"
	"time"
)

var (
	_pm PITManager
)

type PITManager interface {
	Start()
	Get() string
	Shutdown()
}

type pitManager struct {
	index   string
	amount  uint64
	close   chan struct{}
	done    chan struct{}
	mutex   *sync.Mutex
	last    []string
	current []string
	next    []string
	idx     int
}

func (pm *pitManager) Get() string {
	if pm.current == nil || len(pm.current) == 0 {
		return ""
	}

	pm.mutex.Lock()
	pm.idx = pm.idx % len(pm.current)
	pid := pm.current[pm.idx]
	pm.idx++
	pm.mutex.Unlock()

	return pid
}

func (pm *pitManager) Shutdown() {
	close(pm.close)

	<-pm.done
}

func (pm *pitManager) Start() {
	logger.App().Infoln("======================================== PITManager start ========================================")
	defer logger.App().Infoln("======================================== PITManager stop ========================================")

	pm.generateCurrent()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		done := false

		select {
		case <-pm.close:
			{
				done = true
			}
		case now := <-ticker.C:
			{
				if now.Second() == 50 {
					if err := pm.generateNext(); err != nil {
						logger.App().Errorf("generate next error : %s", err.Error())
					}
					continue
				}

				if now.Second() == 0 {
					pm.mutex.Lock()
					if len(pm.next) == 0 {
						pm.mutex.Unlock()
						if err := pm.generateNext(); err != nil {
							logger.App().Errorf("generate next error : %s", err.Error())
						}
						pm.mutex.Lock()
					}
					pm.last = pm.current[:]
					pm.current = pm.next[:]
					pm.mutex.Unlock()
					logger.App().Infof("finish switch next to current : %d - %d", len(pm.last), len(pm.next))
					pm.next = make([]string, 0)
					continue
				}

				if now.Second() == 15 {
					pm.closeLast()
					continue
				}
			}
		}

		if done {
			break
		}
	}

}

func (pm *pitManager) generateCurrent() {
	list, err := generate(pm.index, pm.amount)
	if err != nil {
		logger.App().Error("generate error : %s", err.Error())
		return
	}
	pm.mutex.Lock()
	pm.current = list[:]
	pm.mutex.Unlock()
}

func (pm *pitManager) generateNext() error {
	list := make([]string, 0)
	var err error
	for i := 0; i < 3; i++ {
		list, err = generate(pm.index, pm.amount)
		if err != nil {
			continue
		}
		break
	}
	if err != nil {
		return err
	}

	pm.mutex.Lock()
	pm.next = list[:]
	pm.mutex.Unlock()

	return nil
}

func (pm *pitManager) closeLast() {
	if pm.last != nil && len(pm.last) > 0 {
		if err := closePits(pm.last...); err != nil {
			logger.App().Errorf("close %+v error : %s", pm.last, err.Error())
		}
	}
}

func generate(index string, amount uint64) ([]string, error) {
	if amount == 0 {
		return []string{}, nil
	}

	tmp := make([]string, 0)

	var err error
	for i := 0; i < int(amount); i++ {
		pitRes, opitErr := elasticsearch.Instance().OpenPointInTime([]string{index}, "2m")
		if opitErr != nil {
			err = opitErr
			break
		}
		var pitResponse map[string]interface{}
		if err = json.NewDecoder(pitRes.Body).Decode(&pitResponse); err != nil {
			_ = pitRes.Body.Close()
			break
		}
		id := pitResponse["id"].(string)

		tmp = append(tmp, id)
		_ = pitRes.Body.Close()
	}

	return tmp[:], err
}

func closePits(pits ...string) error {
	if pits == nil || len(pits) == 0 {
		return nil
	}

	var err error

	for _, id := range pits {
		closePitRes, cpitErr := elasticsearch.Instance().ClosePointInTime(
			elasticsearch.Instance().ClosePointInTime.WithBody(
				strings.NewReader(
					fmt.Sprintf("{\"id\":\"%s\"}", id),
				),
			),
		)
		if cpitErr != nil {
			err = cpitErr
			break
		} else {
			if closePitRes.IsError() {
				err = errors.New(fmt.Sprintf("%s : %s", id, closePitRes.String()))
				_ = closePitRes.Body.Close()
				break
			}
		}
	}

	return err
}
