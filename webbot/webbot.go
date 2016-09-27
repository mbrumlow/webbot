package webbot

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os/exec"
	"sync"

	"gopkg.in/mgo.v2/bson"

	"golang.org/x/net/websocket"
)

type Robot interface {
	Stop() error
	Right() error
	Left() error
	Forward() error
	Backward() error
}

type RobotControl int

// Robot Control
const (
	Stop RobotControl = iota
	Left
	Right
	Forward
	Backward
)

type RobotCommand int

const (
	Command RobotCommand = 1 << iota
	StartVideo
	StopVideo
)

const (
	Signal = 1 << iota
	TrackPower
	Video
)

type RobotEvent struct {
	Type  RobotCommand
	Event []byte
}

type WebBot struct {
	controlUrl string
	robot      Robot
	videoDev   string
	pass       string

	// Video related stuff.
	mu           sync.Mutex
	ln           net.Listener
	cmd          *exec.Cmd
	videoRunning bool
}

func New(controlUrl, videoDev, pass string, robot Robot) WebBot {
	return WebBot{controlUrl: controlUrl, pass: pass, robot: robot, videoDev: videoDev}
}

func (wb *WebBot) Run() error {

	wb.robot.Stop()
	defer wb.robot.Stop()

	ws, err := websocket.Dial(wb.controlUrl, "", wb.controlUrl)
	if err != nil {
		return fmt.Errorf("Error connecting: %v", err.Error())
	}
	defer ws.Close()

	if err := wb.authenticate(ws); err != nil {
		return fmt.Errorf("Failed to authenticate: %v", err.Error())
	}

	for {
		data := make([]byte, 0)
		if err := websocket.Message.Receive(ws, &data); err != nil {
			return fmt.Errorf("Error receiving event: %v", err.Error())
		}

		var rb RobotEvent
		if err := bson.Unmarshal(data, &rb); err != nil {
			return fmt.Errorf("Error decoding event: %v", err.Error())
		}

		if err := wb.handleEvent(ws, rb); err != nil {
			return fmt.Errorf("Error handling event: %v", err.Error())
		}
	}

	return nil
}

func (wb *WebBot) authenticate(ws *websocket.Conn) error {

	// Authenticate -- this will improve, this is just to keep the honest people honest.
	if err := websocket.JSON.Send(ws, &wb.pass); err != nil {
		return fmt.Errorf("Failed to send password: %v", err.Error())
	}

	var ok string
	if err := websocket.JSON.Receive(ws, &ok); err != nil {
		return fmt.Errorf("Failed to receive ok from rwc: %v\n", err.Error())
	}

	if ok != "OK" {
		return fmt.Errorf("Password error!")
	}

	return nil
}

func (wb *WebBot) handleEvent(ws *websocket.Conn, ev RobotEvent) error {

	switch ev.Type {
	case Command:
		return wb.handleCommand(ev.Event)
	case StartVideo:
		return wb.StartVideo(ws)
	case StopVideo:
		return wb.StopVideo()
	}

	return fmt.Errorf("Unknown command: %v\n", ev.Type)
}

func (wb *WebBot) handleCommand(e []byte) error {

	var control RobotControl
	if err := json.Unmarshal(e, &control); err != nil {
		return fmt.Errorf("Failed to unmarshal command: %v\n", err.Error())
	}

	switch control {
	case Stop:
		return wb.robot.Stop()
	case Left:
		return wb.robot.Left()
	case Right:
		return wb.robot.Right()
	case Forward:
		return wb.robot.Forward()
	case Backward:
		return wb.robot.Backward()
	}

	return fmt.Errorf("Unknown control: %v\n", control)
}

func (wb *WebBot) StartVideo(ws *websocket.Conn) error {

	log.Printf("Starting video.\n")
	wb.mu.Lock()
	defer wb.mu.Unlock()

	if wb.videoRunning {
		return nil
	}

	wb.videoRunning = true

	ready := make(chan bool, 1)
	go wb.runVideoServer(ws, ready)
	<-ready

	go wb.keepRunning()

	return nil
}

func (wb *WebBot) StopVideo() error {

	// If there is no video it might be a good idea to stop.
	// I will have to think about the location of this logic, I don't like it.
	if err := wb.robot.Stop(); err != nil {
		return err
	}

	log.Printf("Stopping video.\n")
	wb.mu.Lock()
	defer wb.mu.Unlock()

	wb.videoRunning = false

	if wb.cmd != nil {
		wb.cmd.Process.Kill()
		wb.cmd = nil
	}

	if wb.ln != nil {
		wb.ln.Close()
	}

	return nil
}

func (wb *WebBot) keepRunning() {

	first := true
	for {
		wb.mu.Lock()

		if !wb.videoRunning {
			wb.mu.Unlock()
			return
		}

		if !first {
			log.Printf("Restarting video.\n")
		} else {
			first = false
		}

		wb.cmd = exec.Command(
			"ffmpeg", "-s", "640x480", "-f", "video4linux2",
			"-i", wb.videoDev, "-f", "mpeg1video",
			"-r", "30", fmt.Sprintf("tcp://%v", wb.ln.Addr().String()))
		wb.mu.Unlock()

		if err := wb.cmd.Run(); err != nil {
			log.Printf("Video error: %v\n", err.Error())
		}
	}

}

func (wb *WebBot) runVideoServer(ws *websocket.Conn, ready chan bool) error {

	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		return fmt.Errorf("Failed to bind to port: %v", err.Error())
	}

	wb.ln = ln
	ready <- true

	// Non obvious, we really only want one connection...
	for {
		conn, err := ln.Accept()
		if err != nil {
			return fmt.Errorf("Failed to accept connection: %v", err.Error())
			continue
		}

		if err := wb.handleConnection(ws, conn); err != nil {
			return err
		}
	}

}

func (wb *WebBot) handleConnection(ws *websocket.Conn, src net.Conn) error {

	defer src.Close()

	buf := make([]byte, 1024)

	for {
		size, err := src.Read(buf)
		if err != nil {
			log.Printf("Video recive error: %v\n", err.Error())
			break
		}

		if err := wb.videoToWS(ws, buf[0:size]); err != nil {
			return err
		}
	}

	return nil
}

func (wb *WebBot) videoToWS(ws *websocket.Conn, data []byte) error {

	rb := RobotEvent{Type: Video, Event: data}

	if data, err := bson.Marshal(&rb); err != nil {
		return fmt.Errorf("Failed to encode robot event: %v\n", err.Error())
	} else {
		if err := websocket.Message.Send(ws, data); err != nil {
			return fmt.Errorf("Failed to send video to controller: %v", err.Error())
		}
	}

	return nil
}

func (c RobotControl) String() string {

	switch c {
	case Stop:
		return "Stop"
	case Left:
		return "Left"
	case Right:
		return "Right"
	case Forward:
		return "Forward"
	case Backward:
		return "Backward"
	}

	return fmt.Sprintf("COMMAND(<%v>)", c)

}
