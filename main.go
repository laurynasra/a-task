package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
)

type Broker struct {
	Rooms map[string][]chan Message
}

type ChannelRequest struct {
	ch   *chan Message
	room string
}

type Message struct {
	id    int
	event string
	data  string
}

func sendMessage(rw http.ResponseWriter, msg Message) {
	flusher, ok := rw.(http.Flusher)

	if !ok {
		http.Error(rw, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(rw, "id: %v\n", msg.id)
	fmt.Fprintf(rw, "event: %s\n", msg.event)
	fmt.Fprintf(rw, "data: %s\n\n", msg.data)

	flusher.Flush()
}

func main() {
	broker := &Broker{
		Rooms: map[string][]chan Message{},
	}

	router := httprouter.New()

	joinChannel := make(chan *ChannelRequest)
	leaveChannel := make(chan *ChannelRequest)

	go func() {
		for {
			select {
			case jr := <-joinChannel:
				if _, ok := broker.Rooms[jr.room]; ok {
					broker.Rooms[jr.room] = append(broker.Rooms[jr.room], *jr.ch)
				} else {
					roomChannels := make([]chan Message, 0)
					broker.Rooms[jr.room] = append(roomChannels, *jr.ch)
				}
			case lr := <-leaveChannel:
				roomChannels := broker.Rooms[lr.room]
				for i := range roomChannels {
					if roomChannels[i] == *lr.ch {
						broker.Rooms[lr.room] = append(roomChannels[:i], roomChannels[i+1:]...)
						close(*lr.ch)
					}
				}
			}
		}
	}()

	messageId := 0

	router.POST("/infocenter/:room", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rw.Header().Set("Access-Control-Allow-Origin", "*")

		b, _ := ioutil.ReadAll(r.Body)
		room := ps.ByName("room")
		for _, ch := range broker.Rooms[room] {
			ch <- Message{id: messageId, event: "msg", data: string(b)}
		}

		messageId += 1
		rw.WriteHeader(http.StatusNoContent)
	})

	router.GET("/infocenter/:room", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rw.Header().Set("Content-Type", "text/event-stream")
		rw.Header().Set("Cache-Control", "no-cache")
		rw.Header().Set("Connection", "keep-alive")
		rw.Header().Set("Access-Control-Allow-Origin", "*")

		room := ps.ByName("room")
		ch := make(chan Message)
		joinChannel <- &ChannelRequest{room: room, ch: &ch}
		notify := rw.(http.CloseNotifier).CloseNotify()
		tick := time.Tick(time.Second * 10)

		defer func() {
			leaveChannel <- &ChannelRequest{room: room, ch: &ch}
		}()

	Loop:
		for {
			select {
			case msg := <-ch:
				sendMessage(rw, msg)
			case <-notify:
				println("The client closed the connection prematurely. Cleaning up.")
				break Loop
			case <-tick:
				sendMessage(rw, Message{event: "timeout", data: "30s"})
				time.Sleep(1 * time.Second)
				break Loop
			}
		}
	})

	log.Fatal("HTTP Server error: ", http.ListenAndServe(":3000", router))
}
