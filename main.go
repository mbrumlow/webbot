package main

import (
	"bytes"
	"container/list"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/websocket"
)

const (
	maxVideo   = 100
	maxEvents  = 10
	maxChatLog = 100
)

const (
	Signal = 1 << iota
	TrackPower
	Video
	StartVideo
	EndVideo
	ActionEvent
	ChatEvent
	RegisterEvent
)

const (
	_                = iota
	AuthOK           = iota
	AuthError        = iota
	AuthUserInUse    = iota
	AuthPassRequired = iota
	AuthBadPass      = iota
	AuthBadName      = iota
)

// Auth sates
// Used for authentiation flow control.
const (
	authStateGetAuth = iota
	authStateAdd     = iota
	authStatePass    = iota
	authStateOK      = iota
)

type JsonEvent struct {
	Type     int
	Time     int64
	Event    string
	UserInfo UserInfo
}

type Action struct {
	Id     uint64
	Time   string
	Action string
}

type Power struct {
	Left  int16
	Right int16
}

type Chat struct {
	Auth string
	Text string
}

type AuthEvent struct {
	Name  string
	Token string
}

type PassEvent struct {
	Pass string
}

type Client struct {
	To            chan JsonEvent
	From          chan JsonEvent
	Name          string
	Active        bool
	Token         string
	ws            *websocket.Conn
	authenticated bool
}

type UserInfo struct {
	Name string
	Id   string
}

type User struct {
	Name string
	Pass string
}

var (
	clientMu     sync.RWMutex
	clients      = make(map[string]map[*Client]interface{})
	videoClients = make(map[chan []byte]*websocket.Conn)

	chatMu  sync.RWMutex
	chatLog = list.New()

	chatChan = make(chan JsonEvent, 100)

	chatNum uint64

	passMu sync.RWMutex
)

var robothostport = flag.String("host", "", "host port of dartbot.")
var dataDir = flag.String("data", "data", "Data directory.")

func main() {

	flag.Parse()

	if *robothostport == "" {
		flag.PrintDefaults()
		log.Fatal("Plase provide a host:port.\n")
	}

	if err := os.MkdirAll(*dataDir, 0700); err != nil {
		log.Fatal("Failed to create data directory: %v\n", err.Error())
	}

	// Seed the chat number with the curren time.
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	atomic.StoreUint64(&chatNum, uint64(startTime))

	url := fmt.Sprintf("ws://%v/control", *robothostport)
	ref := fmt.Sprintf("http://%v/", *robothostport)

	events := make(chan JsonEvent, 1000)

	go chatDispatcher()
	go startHttp(events)

	for {

		ws, err := websocket.Dial(url, "", ref)
		if err != nil {
			log.Printf("ERROR: Failed to connect to dartbot: %v\n", err.Error())
			robotDownEvent()

			// Clean out pending events.
			for i := len(events); i > 0; i-- {
				<-events
			}

			time.Sleep(1 * time.Second)
			continue
		}

		// Clean out pending events.
		for i := len(events); i > 0; i-- {
			<-events
		}

		go handleRobotEvents(ws)

		for {
			event := <-events
			userInfo := event.UserInfo
			event.UserInfo = UserInfo{}
			if err := websocket.JSON.Send(ws, &event); err != nil {
				log.Printf("ERROR: Failed to send event to robot: %v.\n", err.Error())
				break
			}
			event.UserInfo = userInfo
			sendEventToClient(event)
		}

	}
}

func handleRobotEvents(ws *websocket.Conn) {

	defer ws.Close()

	for {
		var ev JsonEvent
		if err := websocket.JSON.Receive(ws, &ev); err != nil {
			log.Printf("ERROR: failed to recive event from robot: %v\n", err.Error())
			return
		}

		switch ev.Type {
		case Video:
			decodeVideo(ev.Event)
		default:
			log.Println("ERROR: Recived unknown event (%v) from robot.\n", ev.Type)
		}
	}
}

func decodeVideo(s string) {

	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		log.Printf("ERROR: Failed to decode video buffer: %v\n", err.Error())
		return
	}

	sendVideoToClients(decoded)
}

func startHttp(events chan JsonEvent) {

	http.Handle("/video", websocket.Handler(clientVideoHandler))
	http.Handle("/client", websocket.Handler(func(ws *websocket.Conn) {
		clientHandler(ws, events)
	}))

	fs := http.FileServer(http.Dir("webroot"))
	http.Handle("/", http.StripPrefix("/", fs))
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func sendVideoToClients(d []byte) {

	clientMu.RLock()
	defer clientMu.RUnlock()

	for v, ws := range videoClients {

		if len(v) > maxVideo-(maxVideo/10) {
			log.Printf("INFO Dropping video frames on client: %v\n", ws.Request().RemoteAddr)
			for len(v) != 0 {
				<-v
			}
		}

		v <- d
	}
}

func sendEventToClient(ev JsonEvent) {

	switch ev.Type {
	case TrackPower:
		powerEvent(ev.UserInfo, []byte(ev.Event))
	default:
		log.Printf("ERROR: Not sending unknown event type (%v) to server.\n", ev.Type)
	}

}

func powerEvent(userInfo UserInfo, jsonBytes []byte) {

	var p Power
	if err := json.Unmarshal(jsonBytes, &p); err != nil {
		log.Printf("ERROR: Failed to unmarshal power: %v\n", err.Error())
		return
	}

	a := Action{Time: formatedTime(), Action: fmt.Sprintf("-- POWER(%v,%v) --", p.Left, p.Right)}

	jsonBytes, err := json.Marshal(a)
	if err != nil {
		log.Printf("ERROR: Failed to marshal json: %v.\n", err.Error())
	}

	je := JsonEvent{UserInfo: userInfo, Type: ActionEvent, Event: string(jsonBytes)}

	sendToAll(je)

}

func robotDownEvent() {

	a := Action{Time: formatedTime(), Action: "OFFLINE"}

	jsonBytes, err := json.Marshal(a)
	if err != nil {
		log.Printf("ERROR: Failed to marshal json: %v.\n", err.Error())
	}

	je := JsonEvent{UserInfo: UserInfo{Name: "SYSTEM", Id: "SYSTEM:0"}, Type: ActionEvent, Event: string(jsonBytes)}

	sendToAll(je)
}

func jsonEvent(t int, v interface{}, userInfo UserInfo) (JsonEvent, error) {

	jb, err := json.Marshal(v)
	if err != nil {
		return JsonEvent{}, err
	}

	time := time.Now().UnixNano() / int64(time.Millisecond)

	je := JsonEvent{Type: t, Time: time, Event: string(jb), UserInfo: userInfo}
	return je, nil
}

func clientEventReader(c *Client) {
	for {
		var je JsonEvent
		if err := websocket.JSON.Receive(c.ws, &je); err != nil {
			wsLogErrorf(c.ws, "Error reading event: %v", err)
			close(c.From)
			return
		}
		c.From <- je
	}
}

func nameRegistered(name string) (bool, error) {

	userFile := filepath.Join(*dataDir, "users", name)
	_, err := os.Stat(userFile)
	if err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	}

	return false, err
}

func wsAuthClient(ws *websocket.Conn, c *Client, pass string) (bool, error) {

	// TODO -- handle this without a global lock.
	passMu.RLock()
	passMu.RUnlock()

	sha_256 := sha256.New()
	sha_256.Write([]byte(pass))
	ciphertext := hex.EncodeToString(sha_256.Sum(nil))

	userFile := filepath.Join(*dataDir, "users", c.Name)

	b, err := ioutil.ReadFile(userFile)
	if err != nil {
		return false, fmt.Errorf("Failed to read uesr file: %v", err.Error())
	}

	var user User
	if err := json.Unmarshal(b, &user); err != nil {
		return false, fmt.Errorf("Failed to unmarshal user file: %v", err.Error())
	}

	if user.Name == c.Name && user.Pass == ciphertext {
		return true, nil
	}

	if err := wsSendEvent(ws, AuthBadPass, "Bad pass."); err != nil {
		return false, err
	}

	return false, nil
}

func wsAddClient(ws *websocket.Conn, c *Client) (int, error) {

	clientMu.Lock()
	defer clientMu.Unlock()

	tokenMatch := false
	m, ok := clients[c.Name]
	if ok {
		for client, _ := range m {
			if c.Token != "" && client.Token == c.Token {
				tokenMatch = true
			}
		}
	}

	if !ok || ok && !tokenMatch {
		if ok, err := nameRegistered(c.Name); err != nil {
			return 0, err
		} else if ok && !c.authenticated {

			if err := wsSendEvent(ws, AuthPassRequired, "Password Required."); err != nil {
				return 0, err
			}

			return authStatePass, nil
		}
	}

	// A user with this name is already logged in.
	// User is not authenticated.
	// Name is not registered.
	if ok && !tokenMatch && !c.authenticated {

		if err := wsSendEvent(ws, AuthUserInUse, "Name in use."); err != nil {
			return 0, err
		}

		return authStateGetAuth, nil
	}

	// * Nobody else is logged in to this name.
	// * Name not registered.
	if !ok {

		// Create new token for new name.
		if token, err := newToken(); err != nil {
			return AuthError, fmt.Errorf("Failed to create new token: %v", err)
		} else {
			c.Token = token
		}

		m = make(map[*Client]interface{})
		clients[c.Name] = m

	}

	if c.Token == "" {
		if token, err := newToken(); err != nil {
			return AuthError, fmt.Errorf("Failed to create new token: %v", err)
		} else {
			c.Token = token
		}
	}

	m[c] = nil
	log.Printf("Adding client %v:%p\n", c.Name, c)
	return authStateOK, nil
}

func delClient(c *Client) {

	log.Printf("Deleting client %v:%p\n", c.Name, c)

	clientMu.Lock()
	defer clientMu.Unlock()

	m, ok := clients[c.Name]
	if !ok {
		return
	}

	delete(m, c)

	if len(m) == 0 {
		delete(clients, c.Name)
	}

}

func newToken() (string, error) {

	b := make([]byte, 256)
	_, err := rand.Read(b)
	if err != nil {
		return "", fmt.Errorf("Failed to generate new token: %v", err.Error())
	}

	return base64.StdEncoding.EncodeToString(b), nil

}

func wsSendEvent(ws *websocket.Conn, event int, data string) error {

	je, err := jsonEvent(event, data, UserInfo{Name: "SYSTEM", Id: "SYSTEM:0"})
	if err != nil {
		wsLogErrorf(ws, "Failed to create Auth Error event: %v", err)
		return err
	}

	if err := websocket.JSON.Send(ws, &je); err != nil {
		wsLogErrorf(ws, "Failed to send Auth Error event: %v", err)
		return err
	}

	return nil
}

func wsGetAuth(ws *websocket.Conn) (string, string, error) {

	var authEvent AuthEvent
	if err := websocket.JSON.Receive(ws, &authEvent); err != nil {
		return "", "", err
	}

	return authEvent.Name, authEvent.Token, nil
}

func wsGetPass(ws *websocket.Conn) (string, error) {

	var passEvent PassEvent
	if err := websocket.JSON.Receive(ws, &passEvent); err != nil {
		return "", err
	}

	return passEvent.Pass, nil
}

func wsValidateName(ws *websocket.Conn, name string) (bool, error) {

	if name != "" {
		return true, nil
	}

	// TODO: Add more validation requirements.

	wsLogErrorf(ws, "BAD USER: '%v'\n", name)

	if err := wsSendEvent(ws, AuthBadName, "Bad name."); err != nil {
		return false, err
	}

	return false, nil
}

func clientHandler(ws *websocket.Conn, events chan JsonEvent) {

	defer ws.Close()
	wsLogInfo(ws, "Connected.")
	defer wsLogInfo(ws, "Disconnected.")

	client := &Client{
		To:   make(chan JsonEvent, maxEvents),
		From: make(chan JsonEvent, 1),
		ws:   ws,
	}

	state := authStateGetAuth
	for state != authStateOK {
		switch state {

		case authStateGetAuth:
			wsLogInfof(ws, "authStateGetAuth")
			if name, token, err := wsGetAuth(ws); err != nil {
				wsLogErrorf(ws, "Failed to get client auth: %v\n", err.Error())
				return
			} else {
				if ok, err := wsValidateName(ws, name); err != nil {
					wsLogErrorf(ws, "Failed to validate name: %v\n", err.Error())
					return
				} else if ok {
					client.Name = name
					client.Token = token
					state = authStateAdd
				}
			}

		case authStateAdd:
			wsLogInfof(ws, "authStateAdd")
			if newState, err := wsAddClient(ws, client); err != nil {
				wsLogErrorf(ws, "Failed to add client: %v\n", err.Error())
				return
			} else {
				state = newState
			}

		case authStatePass:
			wsLogInfof(ws, "authStatePass")
			if pass, err := wsGetPass(ws); err != nil {
				wsLogErrorf(ws, "Failed to get client pass: %v\n", err.Error())
				return
			} else {
				if ok, err := wsAuthClient(ws, client, pass); err != nil {
					wsLogErrorf(ws, "Failed to auth client: %v\n", err.Error())
					return
				} else if ok {
					client.authenticated = true
					state = authStateAdd
				}
			}
		}
	}

	client.logInfof("Authenticated: %v", client.Token)

	if err := wsSendEvent(ws, AuthOK, client.Token); err != nil {
		client.logErrorf(err.Error())
		return
	}

	go client.chatCatchUp()
	go clientEventReader(client)

	for {
		select {
		case event := <-client.To:
			if err := websocket.JSON.Send(ws, &event); err != nil {
				client.logErrorf("Error sending event: %v", err)
				return
			}
		case clientEvent, ok := <-client.From:
			if !ok {
				return
			}
			client.handleEvent(clientEvent, events)
		}
	}

}

func (c *Client) chatCatchUp() {

	// Get the list while under a lock.
	// Make a copy of the list because we don't want to hold the lock
	// while we wait to send all the events to the client (which my be slow).
	chatMu.RLock()
	catchupLog := make([]JsonEvent, 0, chatLog.Len())
	for e := chatLog.Front(); e != nil; e = e.Next() {
		catchupLog = append(catchupLog, e.Value.(JsonEvent))
	}
	chatMu.RUnlock()

	for _, je := range catchupLog {
		c.To <- je
	}

}

func (c *Client) handleEvent(je JsonEvent, events chan JsonEvent) {

	switch je.Type {
	case ChatEvent:
		c.handleChatEvent(je)
	case TrackPower:
		c.handleTrackPowerEvent(je, events)
	case RegisterEvent:
		c.handleRegisterEvent(je)
	default:
		c.logErrorf("Recived unknown event (%v)\n", je.Type)
	}

}

func (c *Client) handleChatEvent(e JsonEvent) {

	if e.Event == "" {
		return
	}

	if strings.HasPrefix(e.Event, "/") {
		c.handleCommandEvent(e)
		return
	}

	c.logPrefixf("CHAT", "%v\n", e.Event)

	// This will ensure that all chats have a unis id
	id := atomic.AddUint64(&chatNum, 1)

	a := Action{Id: id, Time: formatedTime(), Action: e.Event}
	je, err := jsonEvent(ChatEvent, a, c.userInfo())
	if err != nil {
		c.logErrorf("Failed to create jsonEvent: %v", err)
		return
	}

	// Manage in memory log.
	chatMu.Lock()
	chatLog.PushBack(je)

	for chatLog.Len() > maxChatLog {
		e := chatLog.Front()
		if e != nil {
			chatLog.Remove(e)
		}
	}
	chatMu.Unlock()

	sendToAll(je)
}

func (c *Client) handleRegisterEvent(e JsonEvent) {

	// TODO - validate password.

	// TODO - handle this without a global lock.
	passMu.Lock()
	passMu.Unlock()

	sha_256 := sha256.New()
	sha_256.Write([]byte(e.Event))
	ciphertext := hex.EncodeToString(sha_256.Sum(nil))

	userFile := filepath.Join(*dataDir, "users", c.Name)

	user := User{Name: c.Name, Pass: ciphertext}

	b, err := json.Marshal(&user)
	if err != nil {
		if err := wsSendEvent(c.ws, AuthError, "Internal auth error."); err != nil {
			c.logErrorf("Failed to register: %v\n", err.Error())
		}
		return
	}

	if err := ioutil.WriteFile(userFile, b, 0600); err != nil {
		if err := wsSendEvent(c.ws, AuthError, "Internal auth error."); err != nil {
			c.logErrorf("Failed to register: %v\n", err.Error())
		}
		return
	}

	if err := wsSendEvent(c.ws, AuthOK, c.Token); err != nil {
		c.logErrorf(err.Error())
		return
	}

}

func (c *Client) handleCommandEvent(e JsonEvent) {

	// TODO - command handler.

}

func (c *Client) handleTrackPowerEvent(e JsonEvent, events chan JsonEvent) {

	// Sanity check, decode and encode before sending it to the robot.
	var p Power
	if err := json.Unmarshal([]byte(e.Event), &p); err != nil {
		c.logErrorf("Failed decode TrackPower: %v\n", err)
		return
	}

	c.logPrefixf("POWER", "%v,%v\n", p.Left, p.Right)

	je, err := jsonEvent(TrackPower, p, c.userInfo())
	if err != nil {
		c.logErrorf("Failed to create jsonEvent: %v", err)
		return
	}

	events <- je
}

func (c *Client) logPrefixf(prefix, format string, a ...interface{}) {

	remoteAddr := "0"
	if c.ws != nil {
		remoteAddr = c.ws.Request().RemoteAddr
	}

	msg := fmt.Sprintf(format, a...)
	log.Printf("%v:%v[%p] - %v - %v", remoteAddr, c.Name, c, prefix, msg)

}

func (c *Client) userInfo() UserInfo {
	localPort := "0"
	if c.ws != nil {
		addr := c.ws.Request().RemoteAddr
		hp := strings.Split(addr, ":")
		if len(hp) == 2 {
			localPort = hp[1]
		}
	}

	return UserInfo{Name: c.Name, Id: fmt.Sprintf("%v:%v", c.Name, localPort)}
}

func (c *Client) logInfof(format string, a ...interface{}) {
	c.logPrefixf("INFO", format, a...)
}

func (c *Client) logErrorf(format string, a ...interface{}) {
	c.logPrefixf("ERROR", format, a...)
}

func chatDispatcher() {

	for {
		je := <-chatChan
		sendAll(je)
	}

}

func sendToAll(je JsonEvent) {
	chatChan <- je
}

func sendAll(je JsonEvent) {

	clientMu.RLock()
	defer clientMu.RUnlock()

	for _, m := range clients {
		for c, _ := range m {
			if len(c.To) > maxEvents-(maxEvents/10) {
				c.logErrorf("Dropping events!")
				// TODO: notify the client that some events were dropped.
				for len(c.To) != 0 {
					<-c.To
				}
			}
			c.To <- je
		}
	}
}

func clientVideoHandler(ws *websocket.Conn) {

	if err := sendJSMPHeader(ws); err != nil {
		log.Printf("INFO: Video client ended: %v.\n", err.Error())
		return
	}

	videoChan := make(chan []byte, maxVideo)
	addVideoClient(videoChan, ws)
	defer removeVideoClient(videoChan)

	wsLogInfo(ws, "Video client connected.")
	defer wsLogInfo(ws, "Video client disconnected.")

	for {
		data := <-videoChan
		if err := websocket.Message.Send(ws, data); err != nil {
			wsLogError(ws, err.Error())
			return
		}
	}
}

func addVideoClient(v chan []byte, ws *websocket.Conn) {
	clientMu.Lock()
	defer clientMu.Unlock()
	videoClients[v] = ws
}

func removeVideoClient(v chan []byte) {
	clientMu.Lock()
	defer clientMu.Unlock()
	delete(videoClients, v)
}

func sendJSMPHeader(ws *websocket.Conn) error {

	bb := new(bytes.Buffer)
	bb.Write([]byte("jsmp"))
	binary.Write(bb, binary.BigEndian, uint16(640))
	binary.Write(bb, binary.BigEndian, uint16(480))

	if err := websocket.Message.Send(ws, bb.Bytes()); err != nil {
		return err
	}

	return nil
}

func fixName(name string) string {
	if len(name) > 8 {
		return name[0:8]
	}
	name = name + strings.Repeat(" ", 8-len(name))
	return name
}

// TODO -- fix this logging stuff, its nasty.

func logInfo(r *http.Request, msg string) {
	log.Printf("INFO - %v - %v\n", r.RemoteAddr, msg)
}

func logError(r *http.Request, msg string) {
	log.Printf("ERROR - %v - %v\n", r.RemoteAddr, msg)
}

func wsLogInfo(ws *websocket.Conn, msg string) {
	wsLog(ws, fmt.Sprintf("INFO - %v - %v\n", ws.Request().RemoteAddr, msg))
}

func wsLogInfof(ws *websocket.Conn, format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	wsLog(ws, fmt.Sprintf("INFO - %v - %v\n", ws.Request().RemoteAddr, msg))
}

func wsLogError(ws *websocket.Conn, msg string) {
	wsLog(ws, fmt.Sprintf("ERROR - %v - %v\n", ws.Request().RemoteAddr, msg))
}

func wsLogErrorf(ws *websocket.Conn, format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	wsLog(ws, fmt.Sprintf("ERROR - %v - %v\n", ws.Request().RemoteAddr, msg))
}

func wsLog(ws *websocket.Conn, msg string) {
	log.Printf("%v", msg)
}

func formatedTime() string {
	return time.Now().Format("03:04:05.000")
}
