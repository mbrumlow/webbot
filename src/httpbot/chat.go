package httpbot

import (
	"bytes"
	"container/list"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"webbot"
)

var chatTime uint64
var userCount uint64
var anonCount uint64

type ChatMessage struct {
	Name    string `json:"n"`
	Message string `json:"m"`
	Robot   string `json:"r"`
	Chat    bool   `json:"c"`
}

type ChatHandler struct {
	shared    bool
	lock      sync.RWMutex
	clientMap map[*Robot]bool

	chatMu  sync.RWMutex
	chatLog *list.List
}

func NewChatHandler(shared bool) *ChatHandler {
	return &ChatHandler{
		shared:    shared,
		clientMap: make(map[*Robot]bool),
		chatLog:   list.New(),
	}
}

func (ch *ChatHandler) NextClient(userName string) (string, uint64) {

	userID := atomic.AddUint64(&userCount, 1)

	if len(userName) == 0 {
		anonID := atomic.AddUint64(&anonCount, 1)
		userName = fmt.Sprintf("Anon%v", anonID)
	}

	return userName, userID
}

func (ch *ChatHandler) AddRobot(r *Robot) {
	ch.lock.Lock()
	defer ch.lock.Unlock()
	ch.clientMap[r] = true
	// TODO we need to announce the robot has joined the chat.
}

func (ch *ChatHandler) DelRobot(r *Robot) {
	ch.lock.Lock()
	defer ch.lock.Unlock()
	delete(ch.clientMap, r)
	// TODO we need to announce the robot has left the chat.
}

func (ch *ChatHandler) chat(chat bool, source string, name string, msg string) {

	chatOrder := atomic.AddUint64(&chatTime, 1)

	// We don't need to send the source if it is a shared chat.
	if !ch.shared {
		source = ""
	}

	buf := NewChat(chat, source, name, msg, chatOrder)

	if chat {
		ch.chatMu.Lock()
		ch.chatLog.PushBack(buf)
		for ch.chatLog.Len() > 100 { // TODO make this configurable.
			e := ch.chatLog.Front()
			if e != nil {
				ch.chatLog.Remove(e)
			}
		}
		ch.chatMu.Unlock()
	}

	ch.lock.RLock()
	defer ch.lock.RUnlock()
	for r, _ := range ch.clientMap {
		r.robotForwarder(true, buf)
	}
}

func (ch *ChatHandler) oldChats() [][]byte {

	ch.chatMu.RLock()
	defer ch.chatMu.RUnlock()

	log := make([][]byte, 0, ch.chatLog.Len())
	for e := ch.chatLog.Front(); e != nil; e = e.Next() {
		log = append(log, e.Value.([]byte))
	}

	return log
}

func NewChat(chat bool, source string, name string, msg string, chatOrder uint64) []byte {

	cm := ChatMessage{
		Name:    name,
		Message: msg,
		Robot:   source,
		Chat:    chat,
	}

	j, err := json.Marshal(&cm)
	if err != nil {
		log.Printf("FIXME: handle this error: %v\n", err)
		return nil
	}

	t := webbot.CHAT_CAP

	buf := make([]byte, 0, 4+8+len(j))
	bb := bytes.NewBuffer(buf)

	if err := binary.Write(bb, binary.BigEndian, &t); err != nil {
		panic(err)
	}

	if err := binary.Write(bb, binary.BigEndian, &chatOrder); err != nil {
		panic(err)
	}

	bb.Write(j)

	return bb.Bytes()
}
