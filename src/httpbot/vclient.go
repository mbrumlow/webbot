package httpbot

import (
	"fmt"
	"log"
	"os"
	"util"

	"golang.org/x/net/websocket"
)

type VideoClient struct {
	r       *Robot
	msgChan chan []byte

	debug  bool
	logger *log.Logger
}

func NewVideoClient(r *Robot, maxBuffer int, debug bool) *VideoClient {
	return &VideoClient{
		r:       r,
		msgChan: make(chan []byte, maxBuffer),
		debug:   debug,
		logger:  log.New(os.Stderr, "", log.LstdFlags),
	}
}

func (c *VideoClient) Run(ws *websocket.Conn) {

	errChan := make(chan error, 1)

	go c.messageInHandler(ws, errChan)
	go c.messageOutHandler(ws, errChan)

	c.logf("Registering.\n")
	c.r.videoLock.Lock()
	c.r.videoClients[ws] = c.msgChan
	c.r.videoLock.Unlock()

	err := <-errChan
	c.logf("Handler error: %v\n", err)

	c.logf("Closing socket.\n")
	ws.Close()

	c.logf("Deregistering.\n")
	c.r.videoLock.Lock()
	delete(c.r.videoClients, ws)
	c.r.videoLock.Unlock()

	c.logf("Shutting down byte channel.\n")
	close(c.msgChan)

	c.logf("Waiting for handler.\n")
	for err := range errChan {
		if err == nil {
			break
		} else {
			c.logf("Handler err: %v\n", err)
		}
	}

	c.logf("Client finished!\n")

}

func (c *VideoClient) messageInHandler(ws *websocket.Conn, errChan chan error) {

	c.logf("messageInHanlder started.\n")
	defer c.logf("messageInHandler ended.\n")

	for {
		msg, err := c.ReadMessage(ws, 1)
		if err != nil {
			errChan <- err
			return
		}
		c.logf("Read %v bytes\n", len(msg))
	}
}

func (c *VideoClient) messageOutHandler(ws *websocket.Conn, errChan chan error) {

	c.logf("messageOutHanlder started.\n")
	defer c.logf("messageOutHandler ended.\n")

	hadErr := false
	for {
		select {
		case buf, ok := <-c.msgChan:
			if !ok {
				errChan <- nil
				return
			}

			if !hadErr {
				if err := websocket.Message.Send(ws, buf); err != nil {
					errChan <- err
					hadErr = true
				}
			}
		}
	}
}

func (c *VideoClient) ReadMessage(ws *websocket.Conn, max uint32) ([]byte, error) {
	return util.ReadMessage(ws, 1024)
}

func (c *VideoClient) logf(format string, v ...interface{}) {
	if c.debug && c.logger != nil {
		l := fmt.Sprintf(format, v...)
		c.logger.Printf("V[%p:%p]: %v", c.r, c, l)
	}
}
