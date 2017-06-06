package webbot

import "log"

type CtrlCap struct {
	KeyCode int
	Alt     bool
	Ctrl    bool
	Shift   bool
	Toggle  bool
	Help    string

	id       uint32
	version  uint32
	group    uint32
	revision uint64
	callback func() error
	logger   *log.Logger
	debug    bool
}
