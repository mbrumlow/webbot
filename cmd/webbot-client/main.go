package main

import (
	"flag"
	"fmt"
	"log"
	"time"
	"webbot"

	"net/http"
	_ "net/http/pprof"
)

var host = flag.String("host", "ws://localhost:8888/robot", "Robot Web Control (rwc) host.")
var video = flag.String("video", "/dev/video0", "Path to video device.")
var key = flag.String("key", "", "Api key to webbot host.")

func main() {

	flag.Parse()

	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	ffmpeg := webbot.FFMPEG{
		VideoDriver: "v4l2",
		VideoDev:    *video,
	}

	r := webbot.NewRobot(*host, *key, ffmpeg, true)

	r.AddInfoCap(
		webbot.InfoCap{
			X:       4,
			Y:       20,
			Lable:   "Time",
			Version: 1,
		},
		func() (string, error) {
			return fmt.Sprintf("%v", time.Now()), nil
		},
		1*time.Second,
	)

	count := 0

	r.AddInfoCap(
		webbot.InfoCap{
			X:       4,
			Y:       40,
			Lable:   "Count",
			Version: 1,
		},
		func() (string, error) {
			count++
			s := fmt.Sprintf("%v", count)
			return s, nil
		},
		5*time.Second,
	)

	r.AddCtrlCap(
		webbot.CtrlCap{
			KeyCode: 37,
			Help:    "Move Left",
		},
		1,
		false,
		func() error {
			log.Printf("-- LEFT --\n")
			return nil
		},
	)

	r.AddCtrlCap(
		webbot.CtrlCap{
			KeyCode: 38,
		},
		1,
		false,
		func() error {
			log.Printf("-- FORWARD --\n")
			return nil
		},
	)

	r.AddCtrlCap(
		webbot.CtrlCap{
			KeyCode: 39,
		},
		1,
		false,
		func() error {
			log.Printf("-- RIGHT --\n")
			return nil
		},
	)

	r.AddCtrlCap(
		webbot.CtrlCap{
			KeyCode: 40,
		},
		1,
		false,
		func() error {
			log.Printf("-- BACKWARD --\n")
			return nil
		},
	)

	r.AddCtrlCap(
		webbot.CtrlCap{
			KeyCode: 13,
		},
		1,
		false,
		func() error {
			log.Printf("-- ENTER --\n")
			return nil
		},
	)

	r.AddCtrlCap(
		webbot.CtrlCap{},
		1,
		true,
		func() error {
			log.Printf("-- STOP --\n")
			return nil
		},
	)

	r.AddCtrlCap(
		webbot.CtrlCap{
			KeyCode: 76,
			Alt:     true,
			Toggle:  true,
		},
		0,
		false,
		func() error {
			log.Printf("-- LIGHT-TOGGLE --\n")
			return nil
		},
	)

	r.AddCtrlCap(
		webbot.CtrlCap{
			KeyCode: 65,
			Alt:     true,
			Toggle:  true,
		},
		0,
		false,
		func() error {
			log.Printf("-- FUN-TOGGLE --\n")
			return nil
		},
	)

	for {
		log.Println("--------------------------------------------------------------------------------")
		if err := r.Run(); err != nil {
			log.Printf("Robot Error: %v\n", err)
		}
		time.Sleep(3 * time.Second)
	}
}
