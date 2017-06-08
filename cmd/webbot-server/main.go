package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"httpbot"
	"io"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"golang.org/x/net/websocket"

	_ "net/http/pprof"
)

var userCount = uint64(0)
var anonCount = uint64(0)

func main() {

	key := genKey()

	go func() {
		log.Println(http.ListenAndServe("localhost:6061", nil))
	}()

	hr := httpbot.NewRobot("webbot", 10*time.Second, true)
	http.HandleFunc("/robot", func(w http.ResponseWriter, r *http.Request) {

		log.Printf("Robot connected.\n")
		defer log.Printf("Robot disconnected.\n")

		wsRobotHandler := websocket.Handler(hr.Robot)
		wsRobotHandler.ServeHTTP(w, r)
	})

	http.HandleFunc("/video", func(w http.ResponseWriter, r *http.Request) {

		log.Printf("Video client connected.\n")
		defer log.Printf("Video client disconnected.\n")

		wsRobotHandler := websocket.Handler(hr.Video)
		wsRobotHandler.ServeHTTP(w, r)
	})

	http.HandleFunc("/client", func(w http.ResponseWriter, r *http.Request) {

		userID := atomic.AddUint64(&userCount, 1)
		userName := ""
		cookie := ""

		c, err := r.Cookie("CHAT_NAME")
		if err == nil && len(c.Value) > 0 {
			if n, err := getName(c.Value, key); err == nil {
				userName = n
				cookie = c.Value
			}
		}

		if len(userName) == 0 {
			anonID := atomic.AddUint64(&anonCount, 1)
			userName = fmt.Sprintf("Anon%v", anonID)
		}

		if len(cookie) == 0 {
			if c, err := genCookie(userName, key); err != nil {
				log.Printf("Failed to generate cookie: %v\n", err)
			} else {
				cookie = c
			}
		}

		log.Printf("Client [%v:%v] connected.\n", userID, userName)
		defer log.Printf("Client [%v:%v] disconnected.\n", userID, userName)

		client := httpbot.NewClient(hr, userName, userID, cookie, 100, true)
		wsRobotHandler := websocket.Handler(client.Run)
		wsRobotHandler.ServeHTTP(w, r)
	})

	fs := http.FileServer(http.Dir("webroot"))
	http.Handle("/", http.StripPrefix("/", fs))
	log.Fatal(http.ListenAndServe(":8888", nil))

}

func genCookie(name string, key *[32]byte) (string, error) {

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	_, err = io.ReadFull(rand.Reader, nonce)
	if err != nil {
		return "", err
	}

	ct, err := gcm.Seal(nonce, nonce, []byte(name), nil), nil
	if err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(ct), nil
}

func getName(cookie string, key *[32]byte) (string, error) {

	ciphertext, err := base64.URLEncoding.DecodeString(cookie)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	if len(ciphertext) < gcm.NonceSize() {
		return "", errors.New("malformed ciphertext")
	}

	b, err := gcm.Open(nil,
		ciphertext[:gcm.NonceSize()],
		ciphertext[gcm.NonceSize():],
		nil,
	)
	if err != nil {
		return "", err
	}

	if len(b) == 0 {
		return "", errors.New("Failed to get name.")
	}

	return string(b), nil
}

func genKey() *[32]byte {
	randomKey := &[32]byte{}
	_, err := io.ReadFull(rand.Reader, randomKey[:])
	if err != nil {
		log.Fatal(err)
	}

	return randomKey
}
