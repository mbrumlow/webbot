package main

import (
	"log"

	webbot "../"
)

type textBot struct {
}

func main() {

	r := textBot{}
	wb := webbot.New("ws://localhost:8080/robot", &r)

	wb.Run()
}

func (t *textBot) Stop() error {
	log.Printf("Stop!\n")
	return nil
}

func (t *textBot) Right() error {
	log.Printf("Right!\n")
	return nil
}

func (t *textBot) Left() error {
	log.Printf("Left!\n")
	return nil
}

func (t *textBot) Forward() error {
	log.Printf("Forward!\n")
	return nil
}

func (t *textBot) Backward() error {
	log.Printf("Backward!\n")
	return nil
}
