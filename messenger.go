package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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

type Connection struct {
	Conn   *websocket.Conn
	Id     string
	RoomId string
}

func leaveRoom(connId string, roomId string) {
	idxToKill := -1
	list, ok := connections[roomId]
	if !ok {
		return
	}
	for idx, c := range list {
		if c.Id == connId {
			idxToKill = idx
			break
		}
	}
	if idxToKill >= 0 {
		connections[roomId] = append(list[:idxToKill], list[(idxToKill+1):]...)
	}
	fmt.Printf("\nconnection %s left room: %s", connId, roomId)
}

var inmemory = make(map[string][]Message)
var connections = make(map[string][]Connection)

func main() {

	r := mux.NewRouter()

	r.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, _ := upgrader.Upgrade(w, r, nil)
		connect := &Connection{Conn: conn, Id: xid.New().String()}
		defer func() {
			leaveRoom(connect.Id, connect.RoomId)
			connect.Conn.Close()
			fmt.Printf("\nconnection %s closed", connect.Id)
		}()

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			msgAsString := string(msg)
			var idxEnterRoom = strings.Index(msgAsString, "ENTER ")
			if idxEnterRoom == 0 {
				var roomId = msgAsString[6:]
				connect.RoomId = roomId
				list, ok := connections[roomId]
				if ok {
					connections[roomId] = append(list, *connect)
				} else {
					newlist := []Connection{*connect}
					connections[roomId] = newlist
				}
				fmt.Printf("\nconnection %s entered room: %s", connect.Id, connect.RoomId)
			}

			var idxLeaveRoom = strings.Index(msgAsString, "LEAVE")
			if idxLeaveRoom == 0 {
				leaveRoom(connect.Id, connect.RoomId)
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
		roomMembers, ok := connections[roomId]
		if ok {
			for _, c := range roomMembers {
				closer, _ := c.Conn.NextWriter(websocket.TextMessage)
				json.NewEncoder(closer).Encode(message)
				closer.Close()
			}
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
			http.Error(w, "Room Not found", http.StatusNotFound)
		}
	}).Methods("GET")

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "public/messenger.html")
	})

	http.ListenAndServe(":8080", r)
}
