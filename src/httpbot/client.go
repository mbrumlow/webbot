package httpbot

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"util"
	"webbot"

	"golang.org/x/net/websocket"
)

type Client struct {
	r        *Robot
	msgChan  chan []byte
	groupMap map[uint64]uint64
	name     string
	clientID uint64
	cookie   string

	debug  bool
	logger *log.Logger
}

func NewClient(r *Robot, name string, clientID uint64, cookie string, maxBuffer int, debug bool) *Client {
	return &Client{
		r:        r,
		msgChan:  make(chan []byte, maxBuffer),
		groupMap: make(map[uint64]uint64),
		name:     name,
		clientID: clientID,
		cookie:   cookie,
		debug:    debug,
		logger:   log.New(os.Stderr, "", log.LstdFlags),
	}
}

func (c *Client) Run(ws *websocket.Conn) {

	done := make(chan struct{}, 1)
	wg := sync.WaitGroup{}

	wg.Add(1)
	go c.messageHandler(ws, &wg, done)

	c.sendCookie()

	// Register client.
	c.r.addClient(c)

	c.annouceChat(true)
	defer c.annouceChat(false)

	// Send caps to client.
	capArray := c.r.getCaps()
	for _, msg := range capArray {
		c.sendMessage(msg)
	}

	c.sendUsers()
	c.sendDone()

	for {
		msg, err := c.ReadMessage(ws, 1024)
		if err != nil {
			c.logf("ERROR: %v\n", err)
			break
		}

		if msg, ok := c.handleClientMessage(msg); ok {
			c.r.sendMessage(msg)
		}
	}

	c.logf("Shutting down client message handler.\n")
	close(done)

	c.logf("Waiting for client message handler.\n")
	wg.Wait()

	c.logf("Deregistering client.\n")
	c.r.delClient(c)

	c.logf("Flushing outstanding messages.\n")
	c.flush()

	c.logf("Client finished!\n")
}

func (c *Client) sendCookie() {

	msg := []byte(c.cookie)
	buf := make([]byte, 0, len(msg)+4)
	bb := bytes.NewBuffer(buf)

	t := webbot.COOK_CAP
	if err := binary.Write(bb, binary.BigEndian, &t); err != nil {
		return
	}

	bb.Write(msg)

	{
		revision := atomic.AddUint64(&c.r.capTime, 1)
		msg, err := util.Encode32TimeHeadBuf(t, revision, bb.Bytes())
		if err != nil {
			return
		}

		c.sendMessage(msg)
	}

}

func (c *Client) sendUsers() {

	// Send current list.
	usersBefore := c.r.getUsers()
	for _, msg := range usersBefore {
		c.sendMessage(msg)
	}

	// Get the list again and remove those that are are currently in the list.
	// Leaving us with only those that were here the first round, but no longer
	// here.
	usersAfter := c.r.getUsers()
	for k, _ := range usersAfter {
		delete(usersBefore, k)
	}

	// Tell the client to delete any users that disappeared while sending the
	// users.
	for k, _ := range usersBefore {

		t := webbot.USER_CAP
		name := []byte("none")

		buf := make([]byte, 0, len(name)+8+4)
		bb := bytes.NewBuffer(buf)

		inout := uint32(0)
		if err := binary.Write(bb, binary.BigEndian, &inout); err != nil {
			return
		}

		if err := binary.Write(bb, binary.BigEndian, &k); err != nil {
			return
		}

		bb.Write(name)

		revision := atomic.AddUint64(&c.r.capTime, 1)
		msg, err := util.Encode32TimeHeadBuf(t, revision, bb.Bytes())
		if err != nil {
			return
		}

		c.sendMessage(msg)
	}
}

func (c *Client) sendDone() {

	c.logf("Sending done!\n")

	t := webbot.DONE_CAP
	buf := make([]byte, 0, 4)
	bb := bytes.NewBuffer(buf)

	revision := atomic.AddUint64(&c.r.capTime, 1)
	msg, err := util.Encode32TimeHeadBuf(t, revision, bb.Bytes())
	if err != nil {
		return
	}

	c.sendMessage(msg)

}

func (c *Client) handleClientMessage(msg []byte) ([]byte, bool) {

	bb := bytes.NewBuffer(msg)

	var t uint32
	if err := binary.Read(bb, binary.BigEndian, &t); err != nil {
		return nil, false
	}

	switch t {
	case webbot.CTRL_CAP:
		return c.handleClientCtrlCap(bb.Bytes())
	case webbot.CHAT_CAP:
		return c.handleClientChatCap(bb.Bytes())
	default:
		c.logf("Received unknown message type (%v) from web client\n", t)
	}

	return nil, false
}

func (c *Client) annouceChat(in bool) {

	t := webbot.USER_CAP

	msg := []byte(c.name)
	buf := make([]byte, 0, len(msg)+8+4)
	bb := bytes.NewBuffer(buf)

	inout := uint32(0)
	if in {
		inout = uint32(1)
	}

	if err := binary.Write(bb, binary.BigEndian, &inout); err != nil {
		return
	}

	if err := binary.Write(bb, binary.BigEndian, &c.clientID); err != nil {
		return
	}

	bb.Write(msg)

	{
		revision := atomic.AddUint64(&c.r.capTime, 1)
		msg, err := util.Encode32TimeHeadBuf(t, revision, bb.Bytes())
		if err != nil {
			return
		}

		c.r.robotForwarder(true, msg)

		c.r.capLock.Lock()
		defer c.r.capLock.Unlock()

		if in {
			c.r.userList[c.clientID] = msg
		} else {
			delete(c.r.userList, c.clientID)
		}

	}
}

func (c *Client) handleClientChatCap(msg []byte) ([]byte, bool) {

	t := webbot.CHAT_CAP

	buf := make([]byte, 0, 4+8+8+len(msg))
	bb := bytes.NewBuffer(buf)

	if err := binary.Write(bb, binary.BigEndian, &t); err != nil {
		return nil, false
	}

	if err := binary.Write(bb, binary.BigEndian, &c.clientID); err != nil {
		return nil, false
	}

	chatOrder := atomic.AddUint64(&c.r.chatOrder, 1)
	if err := binary.Write(bb, binary.BigEndian, &chatOrder); err != nil {
		return nil, false
	}

	// TODO encode timestamps.

	bb.Write(msg)

	c.r.robotForwarder(true, bb.Bytes())
	return nil, false
}

func (c *Client) handleClientCtrlCap(msg []byte) ([]byte, bool) {

	t := webbot.CTRL_CAP
	bb := bytes.NewBuffer(msg)

	var id uint32
	if err := binary.Read(bb, binary.BigEndian, &id); err != nil {
		return nil, false
	}

	var down uint32
	if err := binary.Read(bb, binary.BigEndian, &down); err != nil {
		return nil, false
	}

	i, g, ok, err := c.r.groupFilter(c, id, down)
	if err != nil {
		c.logf("ERROR: %v\n", err)
		return nil, false
	} else if !ok {
		return nil, false
	}

	buf := make([]byte, 0, 12)
	bb = bytes.NewBuffer(buf)

	if err := binary.Write(bb, binary.BigEndian, &t); err != nil {
		return nil, false
	}
	if err := binary.Write(bb, binary.BigEndian, &g); err != nil {
		return nil, false
	}
	if err := binary.Write(bb, binary.BigEndian, &i); err != nil {
		return nil, false
	}

	c.logf("CTRL_CAP: Group: %v, Id: %v\n", g, i)

	return bb.Bytes(), true
}

func (c *Client) ReadMessage(ws *websocket.Conn, max uint32) ([]byte, error) {

	msg, err := util.ReadMessage(ws, 1024)

	// TODO Sanitize messages.
	// 1) Convert simple chat message into complex one.
	// size:string -> id:size:string

	return msg, err

}

func (c *Client) groupFilter(group, id, down uint32) bool {

	key := uint64(group)
	key = key<<32 | uint64(id)

	if down > 0 {
		c.groupMap[key] += 1
	} else {
		c.groupMap[key] -= 1
	}

	if c.groupMap[key] <= 1 {
		return true
	}

	return false
}

func (c *Client) sendMessage(msg []byte) {
	// TODO: handle slow clients, we can't have them lagging us.
	// NOTE: The entire server might just be slow not the client.
	c.msgChan <- msg
}

func (c *Client) messageHandler(ws *websocket.Conn, wg *sync.WaitGroup, done <-chan struct{}) {

	c.logf("messageHandler started.\n")
	defer c.logf("messageHandler ended.\n")

	defer wg.Done()
	for {
		select {
		case msg := <-c.msgChan:
			websocket.Message.Send(ws, msg)
		case <-done:
			return
		}
	}
}

func (c *Client) flush() {
	close(c.msgChan)
	util.DrainChan(c.msgChan, func() {})
}

func (c *Client) logf(format string, v ...interface{}) {
	if c.debug && c.logger != nil {
		l := fmt.Sprintf(format, v...)
		c.logger.Printf("C[%p:%v]: %v", c.r, c.name, l)
	}
}
