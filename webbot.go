package webbot

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os/exec"
	"sync"

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
	Event string
}

type WebBot struct {
	controlUrl string
	robot      Robot
	videoDev   string

	// Video related stuff.
	mu               sync.Mutex
	ln               net.Listener
	cmd              *exec.Cmd
	keepVideoRunning bool
}

func New(controlUrl string, videoDev string, robot Robot) WebBot {
	return WebBot{controlUrl: controlUrl, robot: robot, videoDev: videoDev}
}

func (wb *WebBot) Run() error {

	wb.robot.Stop()
	defer wb.robot.Stop()

	ws, err := websocket.Dial(wb.controlUrl, "", wb.controlUrl)
	if err != nil {
		return fmt.Errorf("Error connecting: %v", err.Error())
	}
	defer ws.Close()

	for {
		var ev RobotEvent
		if err := websocket.JSON.Receive(ws, &ev); err != nil {
			return fmt.Errorf("Error reciving event: %v", err.Error())
		}

		if err := wb.handleEvent(ws, ev); err != nil {
			return fmt.Errorf("Error handling event: %v", err.Error())
		}
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

func (wb *WebBot) handleCommand(e string) error {

	var control RobotControl
	if err := json.Unmarshal([]byte(e), &control); err != nil {
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

	return fmt.Errorf("Unknown contro: %v\n", control)
}

func (wb *WebBot) StartVideo(ws *websocket.Conn) error {

	wb.mu.Lock()
	defer wb.mu.Unlock()

	wb.keepVideoRunning = true

	ready := make(chan bool, 1)
	go wb.runVideoServer(ws, ready)
	<-ready

	go wb.keepRunning()

	return nil
}

func (wb *WebBot) StopVideo() error {

	wb.mu.Lock()
	defer wb.mu.Unlock()

	wb.keepVideoRunning = false

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

	for {
		wb.mu.Lock()

		if !wb.keepVideoRunning {
			wb.mu.Unlock()
			return
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
		return fmt.Errorf("Faied to bind to port: %v", err.Error())
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

	// KILL ME NOW.
	// TODO - Lets not piggy back off the websocket. Lets make a real
	// tcp connection so we don't have to do this silly mess.

	encoded := base64.StdEncoding.EncodeToString(data)

	event := RobotEvent{Type: Video, Event: encoded}

	if err := websocket.JSON.Send(ws, &event); err != nil {
		return fmt.Errorf("Failed to send video to controler: %v", err.Error())
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
