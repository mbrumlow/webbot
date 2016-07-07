package webbot

import (
	"encoding/json"
	"fmt"

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

type RobotEvent struct {
	Type  RobotCommand
	Event string
}

type WebBot struct {
	controlUrl string
	robot      Robot
}

func New(controlUrl string, robot Robot) WebBot {
	return WebBot{controlUrl: controlUrl, robot: robot}
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

		if err := wb.handleEvent(ev); err != nil {
			return fmt.Errorf("Error handling event: %v", err.Error())
		}
	}

	return nil
}

func (wb *WebBot) handleEvent(ev RobotEvent) error {

	switch ev.Type {
	case Command:
		return wb.handleCommand(ev.Event)
	case StartVideo:
		return wb.robot.StartVideo()
	case StopVideo:
		return wb.robot.StopVideo()
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
