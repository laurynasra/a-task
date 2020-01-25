package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
)

type Broker struct {
	Rooms map[string][]chan string
}

type JoinRequest struct {
	ch   *chan string
	room string
}

func main() {
	broker := &Broker{
		Rooms: map[string][]chan string{},
	}

	router := httprouter.New()

	joinChannel := make(chan *JoinRequest, 1)
	leaveChannel := make(chan string, 1)

	go func() {
		for {
			select {
			case jr := <-joinChannel:
				if _, ok := broker.Rooms[jr.room]; ok {
					broker.Rooms[jr.room] = append(broker.Rooms["baras"], *jr.ch)
				} else {
					roomChannels := make([]chan string, 0)
					broker.Rooms[jr.room] = append(roomChannels, *jr.ch)
				}
			case <-leaveChannel:
			}
		}
	}()

	// ch := make(chan string)

	router.POST("/infocentras/:room", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		for _, ch := range broker.Rooms["baras"] {
			ch <- time.Now().Format(time.RFC850)
		}
		rw.WriteHeader(http.StatusCreated)
	})

	router.GET("/infocentras/:room", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		flusher, ok := rw.(http.Flusher)

		if !ok {
			http.Error(rw, "Streaming unsupported!", http.StatusInternalServerError)
			return
		}

		room := ps.ByName("room")
		ch := make(chan string)
		joinChannel <- &JoinRequest{room: room, ch: &ch}
		tick := time.Tick(time.Second * 30)

		rw.Header().Set("Content-Type", "text/event-stream")
		rw.Header().Set("Cache-Control", "no-cache")
		rw.Header().Set("Connection", "keep-alive")
		rw.Header().Set("Access-Control-Allow-Origin", "*")

		fmt.Fprintf(rw, "data: %s\n\n", "labas")
		flusher.Flush()

	Loop:
		for {
			select {
			case msg := <-ch:
				fmt.Fprintf(rw, "data: %s\n\n", msg)
				flusher.Flush()
			case <-tick:
				close(ch)
				// broker.Rooms[room] =
				break Loop
			}
		}

		fmt.Fprintf(rw, "data: %s\n\n", "timeout")
		flusher.Flush()
	})

	log.Fatal("HTTP Server error: ", http.ListenAndServe(":3000", router))
}
