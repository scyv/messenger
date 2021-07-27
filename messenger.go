package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/rs/xid"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Room struct {
	RoomId string `json:"roomId"`
	Name   string `json:"name"`
}
type Message struct {
	MessageId string    `json:"messageId"`
	RoomId    string    `json:"roomId"`
	Sender    string    `json:"sender"`
	Text      string    `json:"text"`
	Data      string    `json:"data"`
	Timestamp time.Time `json:"timestamp"`
}

func main() {

	var inmemory = make(map[string][]Message)

	r := mux.NewRouter()

	r.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, _ := upgrader.Upgrade(w, r, nil)

		for {
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			fmt.Printf("\n%s sent: %s", conn.RemoteAddr(), string(msg))
			if err = conn.WriteMessage(msgType, msg); err != nil {
				return
			}
		}
	})

	r.HandleFunc("/room/{roomId}/messages", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		roomId := vars["roomId"]

		var message Message

		json.NewDecoder(r.Body).Decode(&message)
		message.MessageId = xid.New().String()
		message.RoomId = roomId
		message.Timestamp = time.Now()

		list, ok := inmemory[roomId]
		if ok {
			inmemory[roomId] = append(list, message)
		} else {
			newlist := []Message{message}
			inmemory[roomId] = newlist
		}
		fmt.Printf("\nMessage from %s with id %s", message.Sender, message.MessageId)
	}).Methods("POST")

	r.HandleFunc("/room/{roomId}/messages", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		roomId := vars["roomId"]

		list, ok := inmemory[roomId]
		if ok {
			json.NewEncoder(w).Encode(list)
		} else {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, "NOT_FOUND")
		}
	}).Methods("GET")

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "public/messenger.html")
	})

	http.ListenAndServe(":8080", r)
}
