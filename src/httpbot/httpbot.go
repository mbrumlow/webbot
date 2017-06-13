package httpbot

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"
	"util"
	"webbot"

	"golang.org/x/net/websocket"
)

type Robot struct {
	name    string
	timeout time.Duration

	clientLock sync.RWMutex
	clients    map[*Client]bool

	capTime  uint64
	capLock  sync.RWMutex
	capMap   map[uint32][]byte
	capCache map[uint32][]byte
	capGroup map[uint32]uint32

	userLock sync.RWMutex
	userList map[uint64][]byte

	groupLock sync.Mutex
	groupMap  map[uint32]map[uint32]*clog
	groupLast map[uint32]uint32
	logCount  uint64

	chatOrder uint64

	videoLock    sync.RWMutex
	videoClients map[*websocket.Conn]chan []byte

	sessionLock sync.Mutex
	locked      bool

	mu      sync.RWMutex
	msgChan chan []byte
	active  bool

	debug  bool
	logger *log.Logger
}

type clog struct {
	count uint64
	log   uint64
}

func NewRobot(name string, timeout time.Duration, debug bool) *Robot {
	r := &Robot{
		name:         name,
		timeout:      timeout,
		capMap:       make(map[uint32][]byte),
		capCache:     make(map[uint32][]byte),
		capGroup:     make(map[uint32]uint32),
		userList:     make(map[uint64][]byte),
		groupMap:     make(map[uint32]map[uint32]*clog),
		groupLast:    make(map[uint32]uint32),
		clients:      make(map[*Client]bool),
		videoClients: make(map[*websocket.Conn]chan []byte),
		debug:        debug,
		logger:       log.New(os.Stderr, "", log.LstdFlags),
	}

	// Initialize with robot offline.
	r.robotCapHandler(true, webbot.OFFLINE_CAP, make([]byte, 0))

	atomic.StoreUint64(&r.chatOrder, uint64(time.Now().Unix()))

	return r
}

// To robot -- single connection to robot.
func (r *Robot) Robot(ws *websocket.Conn) {

	r.logf("Robot started!\n")
	defer r.logf("Robot Finished!\n")

	// Acquire robot session lock.
	if ok := r.lock(); !ok {
		// TODO: send error back letting robot know it can't connect at this
		// time.
		return
	} else {
		defer r.unlock()
	}

	r.capLock.Lock()
	r.capMap = make(map[uint32][]byte)
	r.capCache = make(map[uint32][]byte)
	r.capGroup = make(map[uint32]uint32)
	r.capLock.Unlock()

	r.groupLock.Lock()
	r.groupMap = make(map[uint32]map[uint32]*clog)
	r.groupLast = make(map[uint32]uint32)
	r.groupLock.Unlock()

	// Let the clients know that the robot is coming on line.
	{
		forward, buf := r.robotCapHandler(true, webbot.ONLINE_CAP, make([]byte, 0))
		r.robotForwarder(forward, buf)
	}

	// Setup new channel for everybody to use.
	r.mu.Lock()
	r.msgChan = make(chan []byte, 0)
	r.active = true
	r.mu.Unlock()

	done := make(chan struct{}, 1)
	errChan := make(chan error, 1)

	go r.fromRobot(ws, errChan)
	go r.toRobot(done, ws, errChan)

	r.clientLock.Lock()
	if len(r.clients) > 0 {
		r.enableVideo(true)
	}
	r.clientLock.Unlock()

	err := <-errChan
	r.logf("Handler error: %v\n", err)

	r.logf("Closing socket.\n")
	ws.Close()

	// Start the shutdown process.
	close(done)

	r.mu.Lock()
	r.active = false
	r.mu.Unlock()

	r.logf("Draining message channel.\n")
	close(r.msgChan)
	util.DrainChan(r.msgChan, func() {})

	r.logf("Waiting for handler.\n")
	<-errChan

	r.capLock.Lock()
	r.capMap = make(map[uint32][]byte)
	r.capCache = make(map[uint32][]byte)
	r.capGroup = make(map[uint32]uint32)
	r.capLock.Unlock()

	r.groupLock.Lock()
	r.groupMap = make(map[uint32]map[uint32]*clog)
	r.groupLast = make(map[uint32]uint32)
	r.groupLock.Unlock()

	forward, buf := r.robotCapHandler(true, 2, make([]byte, 0))
	r.robotForwarder(forward, buf)
}

func (r *Robot) lock() bool {
	r.sessionLock.Lock()
	defer r.sessionLock.Unlock()
	if r.locked {
		return false
	}
	r.locked = true
	return true
}

func (r *Robot) unlock() {
	r.sessionLock.Lock()
	defer r.sessionLock.Unlock()
	r.locked = false
}

func (r *Robot) fromRobot(ws *websocket.Conn, errChan chan error) {
	for {

		if err := ws.SetReadDeadline(time.Now().Add(r.timeout)); err != nil {
			errChan <- err
			return
		}

		msg, err := util.ReadMessage(ws, 4096)
		if err != nil {
			errChan <- err
			return
		}

		var id uint32
		bb := bytes.NewBuffer(msg)
		if err := binary.Read(bb, binary.BigEndian, &id); err != nil {
			errChan <- err
			return
		}

		forward, buf := r.robotVideoHandler(true, id, bb.Bytes())
		forward, buf = r.robotCapHandler(forward, id, buf)
		r.robotForwarder(forward, buf)
	}
}

func (r *Robot) toRobot(done <-chan struct{}, ws *websocket.Conn, errChan chan error) {
	for {
		select {
		case msg := <-r.msgChan:
			if err := ws.SetWriteDeadline(time.Now().Add(r.timeout)); err != nil {
				errChan <- err
				return
			}
			if err := r.writeMsg(ws, msg); err != nil {
				errChan <- err
				return
			}
		case <-done:
			errChan <- nil
			return
		}
	}

}

func (r *Robot) relayVideo(buf []byte) {

	r.videoLock.RLock()
	defer r.videoLock.RUnlock()

	for _, v := range r.videoClients {
		if len(v) < 100 {
			v <- buf
		} else {
			r.logf("Dropping frame on video client.")
		}

	}
}

// To clients wanting to view the video stream.
func (r *Robot) Video(ws *websocket.Conn) {

	byteChan := make(chan []byte, 1024)

	r.logf("Setting up video client.\n")
	done := make(chan interface{}, 1)
	go func() {
		for {
			select {
			case buf := <-byteChan:
				if err := websocket.Message.Send(ws, buf); err != nil {
					r.logf("Video client write error: %v\n", err)
					return
				}
			case <-done:
				done <- true
				return
			}
		}
	}()

	r.logf("Registering video client.\n")
	r.videoLock.Lock()
	r.videoClients[ws] = byteChan
	r.videoLock.Unlock()

	buf := make([]byte, 1)
	for {
		n, err := ws.Read(buf)
		if err != nil {
			r.logf("Video client read error: %v\n", err)
			break
		} else {
			r.logf("Video client read %v bytes\n", n)
		}
	}

	r.logf("Deregistering video client.\n")
	r.videoLock.Lock()
	delete(r.videoClients, ws)
	r.videoLock.Unlock()

	r.logf("Shutting down video client.\n")
	done <- true

	r.logf("Waiting for video client to shutdown.\n")
	<-done

}

func (r *Robot) getUsers() map[uint64][]byte {

	r.userLock.RLock()
	defer r.userLock.RUnlock()

	userList := make(map[uint64][]byte)
	for k, v := range r.userList {
		userList[k] = v
	}

	return userList
}

func (r *Robot) getCaps() [][]byte {

	r.capLock.RLock()
	defer r.capLock.RUnlock()

	capArray := make([][]byte, 0)
	for _, v := range r.capMap {
		capArray = append(capArray, v)
	}
	for _, v := range r.capCache {
		capArray = append(capArray, v)
	}

	return capArray
}

func (r *Robot) groupFilter(c *Client, id, down uint32) (uint32, uint32, bool, error) {

	r.capLock.RLock()
	defer r.capLock.RUnlock()

	// Validate this is a ctrlcap.
	if _, ok := r.capMap[id]; !ok {
		return 0, 0, false, fmt.Errorf("Invalid ctrlcap %v.", id)
	}

	// Get ctrlcaps group.
	group, ok := r.capGroup[id]
	if !ok {
		return 0, 0, false, fmt.Errorf("Missing group for ctrlcap %v.", id)
	}

	if group == uint32(0) {
		return id, group, true, nil
	}

	// Prevents client from getting more than one vote!
	if !c.groupFilter(group, id, down) {
		return 0, 0, false, nil
	}

	r.groupLock.Lock()
	defer r.groupLock.Unlock()

	cl, ok := r.groupMap[group][id]
	if !ok {
		cl = &clog{}
	}
	r.groupMap[group][id] = cl

	if down > 0 {
		cl.count += 1
		r.logCount += 1
		cl.log = r.logCount
	} else if cl.count > 0 {
		cl.count -= 1
		r.logCount += 1
		cl.log = r.logCount
	}

	cl = &clog{}
	maxk := uint32(0)
	for k, v := range r.groupMap[group] {
		if v.count > 0 && v.count >= cl.count && v.log >= cl.log {
			cl = v
			maxk = k
		}
	}

	// This ctrlcap is already active for this group.
	if r.groupLast[group] == maxk {
		return 0, 0, false, nil
	} else {
		r.groupLast[group] = maxk
	}

	return maxk, group, true, nil
}

func (r *Robot) addClient(c *Client) {
	r.clientLock.Lock()
	defer r.clientLock.Unlock()

	if len(r.clients) == 0 {
		r.enableVideo(true)
	}

	r.clients[c] = true
}

func (r *Robot) delClient(c *Client) {
	r.clientLock.Lock()
	defer r.clientLock.Unlock()
	delete(r.clients, c)

	if len(r.clients) == 0 {
		r.enableVideo(false)
	}
}

func (r *Robot) enableVideo(enable bool) {

	r.logf("Video enable: %v\n", enable)

	buf := make([]byte, 0, 8)
	bb := bytes.NewBuffer(buf)

	t := webbot.VIDEO_CAP
	if err := binary.Write(bb, binary.BigEndian, &t); err != nil {
		return
	}

	e := uint32(0)
	if enable {
		e = uint32(1)
	}

	if err := binary.Write(bb, binary.BigEndian, &e); err != nil {
		return
	}

	r.sendMessage(bb.Bytes())
}

// Handles incoming video streams and multiplexes them out to active clients.
func (r *Robot) robotVideoHandler(forward bool, id uint32, buf []byte) (bool, []byte) {
	return true, buf
}

// Handles dealing with any custom events that have server side requirements.
func (r *Robot) robotCapHandler(forward bool, t uint32, buf []byte) (bool, []byte) {

	switch t {
	case webbot.PING_CAP:
		return false, nil
	case webbot.VIDEO_BUF_CAP:
		r.relayVideo(buf)
		return false, nil
	case webbot.INFO_CAP:
		fallthrough
	case webbot.CTRL_CAP:

		bb := bytes.NewBuffer(buf)

		var id uint32
		if err := binary.Read(bb, binary.BigEndian, &id); err != nil {
			// If this fails we are already in trouble.
			panic(err)
		}

		var version uint32
		if err := binary.Read(bb, binary.BigEndian, &version); err != nil {
			// If this fails we are already in trouble.
			panic(err)
		}

		var group uint32
		if err := binary.Read(bb, binary.BigEndian, &group); err != nil {
			// If this fails we are already in trouble.
			panic(err)
		}

		var revision uint64
		if err := binary.Read(bb, binary.BigEndian, &revision); err != nil {
			// If this fails we are already in trouble.
			panic(err)
		}

		revision = atomic.AddUint64(&r.capTime, 1)
		if msg, err := util.EncodeCapDef(t, id, version, group, revision, bb.Bytes()); err != nil {
			// If util.Encode32TimeHeadBuf fails we have much bigger problems.
			panic(err)
		} else {
			r.capLock.Lock()
			r.capMap[id] = msg

			if t == webbot.CTRL_CAP {
				r.setupCtrlGroup(id, group)
			}

			r.capLock.Unlock()

			return true, msg
		}

	}

	revision := atomic.AddUint64(&r.capTime, 1)
	msg, err := util.Encode32TimeHeadBuf(t, revision, buf)
	if err != nil {
		// If util.Encode32TimeHeadBuf fails we have much bigger problems.
		panic(err)
	}

	r.capLock.Lock()
	r.capCache[t] = msg
	r.capLock.Unlock()

	return true, msg
}

func (r *Robot) setupCtrlGroup(id, ng uint32) {

	r.groupLock.Lock()
	defer r.groupLock.Unlock()

	ngm, ok := r.groupMap[ng]
	if !ok {
		ngm = make(map[uint32]*clog)
		r.groupMap[ng] = ngm
	}

	og, ok := r.capGroup[id]
	r.capGroup[id] = ng
	if !ok {
		return
	}

	ogm, ok := r.groupMap[og]
	if !ok {
		return
	}

	ocl := ogm[id]
	ncl := ngm[id]

	ncl.count += ocl.count
	ncl.log = ocl.log

	delete(ogm, id)

}

func (r *Robot) sendMessage(msg []byte) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.active {
		r.msgChan <- msg
	}
}

// Handles forwarding events to web clients.
func (r *Robot) robotForwarder(forward bool, buf []byte) {

	if !forward {
		return
	}

	r.clientLock.RLock()
	defer r.clientLock.RUnlock()

	for k, _ := range r.clients {
		k.sendMessage(buf)
	}
}

func (r *Robot) writeMsg(ws *websocket.Conn, msg []byte) error {
	return util.WriteMessage(ws, msg)
}

func (r *Robot) logf(format string, v ...interface{}) {
	if r.debug && r.logger != nil {
		l := fmt.Sprintf(format, v...)
		r.logger.Printf("R[%v]: %v", r.name, l)
	}
}
