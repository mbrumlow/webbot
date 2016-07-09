package main

import (
	"flag"
	"log"
	"time"

	webbot "../"
)

type textBot struct {
}

var host = flag.String("host", "ws://localhost:8080/robot", "Robot Web Control (rwc) host.")
var video = flag.String("video", "/dev/video0", "Path to video device.")
var pass = flag.String("pass", "", "Password to rwc host")

func main() {

	flag.Parse()

	r := textBot{}
	wb := webbot.New(*host, *video, *pass, &r)

	for {
		if err := wb.Run(); err != nil {
			log.Printf("RUN ERROR: %v\n", err.Error())
		}
		time.Sleep(1 * time.Second)
	}
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
