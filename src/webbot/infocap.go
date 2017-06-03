package webbot

import (
	"log"
	"sync"
	"time"
	"util"
)

type InfoCap struct {
	X       int
	Y       int
	Version int
	Lable   string

	id       uint32
	version  uint32
	group    uint32
	revision uint64
	callback func() (string, error)
	interval time.Duration
	logger   *log.Logger
	debug    bool
}

func (ic *InfoCap) Run(wg *sync.WaitGroup, msgChan chan []byte, done <-chan struct{}) {

	defer wg.Done()

	ic.execute(msgChan)

	work := true
	for work {
		func() {
			t := time.NewTimer(ic.interval)
			defer t.Stop()
			select {
			case <-t.C:
				ic.execute(msgChan)
			case <-done:
				work = false
				return
			}
		}()
	}
}

func (ic *InfoCap) execute(msgChan chan []byte) {
	s, err := ic.callback()
	if err != nil {
		ic.logf("INFOCAP[%v:%v]: ERROR: %v\n", ic.id, ic.Lable, err.Error())
	} else {
		if buf, err := util.Encode32HeadBuf(ic.id, []byte(s)); err != nil {
			ic.logf("INFOCAP[%v:%v]: ERROR: %v\n", ic.id, ic.Lable, err.Error())
		} else {
			msgChan <- buf
		}
	}
}

func (ic *InfoCap) logf(format string, v ...interface{}) {
	if ic.debug && ic.logger != nil {
		ic.logger.Printf(format, v...)
	}
}
