package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/rs/xid"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
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

var adminToken = ""
var inmemory = make(map[string][]Message)
var connections = make(map[string][]Connection)

const maxRooms = 100
const maxMessages = 1000

func subscribeRoom(connection *Connection, roomId string) {
	connection.RoomId = roomId
	list, ok := connections[roomId]
	if ok {
		connections[roomId] = append(list, *connection)
	} else {
		newlist := []Connection{*connection}
		connections[roomId] = newlist
	}
	fmt.Printf("\nconnection %s subscribed room: %s", connection.Id, connection.RoomId)
}

func unsubscribeRoom(connId string, roomId string) {
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
	fmt.Printf("\nconnection %s unsubscribed room: %s", connId, roomId)
}

func saveMessage(message *Message) {
	roomId := message.RoomId

	list, ok := inmemory[roomId]
	if ok {
		if len(list) >= maxMessages {
			inmemory[roomId] = append(list[1:], *message)
		} else {
			inmemory[roomId] = append(list, *message)
		}
	} else {
		if len(inmemory) >= maxRooms {
			fmt.Printf("\nNo more rooms allowed (%d)", len(inmemory))
			return
		}
		newlist := []Message{*message}
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
}

func checkRoomId(roomId string) bool {
	matches, _ := regexp.MatchString("^[a-zA-Z0-9-_]+$", roomId)
	return matches
}

func main() {

	r := mux.NewRouter()

	r.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			return
		}
		conn, err := upgrader.Upgrade(w, r, w.Header())
		if conn == nil || err != nil {
			fmt.Printf("\nError: %s", err)
			return
		}
		connect := &Connection{Conn: conn, Id: xid.New().String()}
		defer func() {
			if connect != nil && connect.Conn != nil && conn != nil {
				unsubscribeRoom(connect.Id, connect.RoomId)
				connect.Conn.Close()
				fmt.Printf("\nconnection %s closed", connect.Id)
			}
		}()

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				fmt.Printf("\nError: %s", err)
				return
			}
			msgAsString := string(msg)
			var idxSubscribeRoom = strings.Index(msgAsString, "SUBSCRIBE ")
			var idxUnsubscribeRoom = strings.Index(msgAsString, "UNSUBSCRIBE")
			if idxSubscribeRoom == 0 {
				var roomId = msgAsString[10:]
				if !checkRoomId(roomId) {
					return
				}
				subscribeRoom(connect, roomId)
			} else if idxUnsubscribeRoom == 0 {
				unsubscribeRoom(connect.Id, connect.RoomId)
			} else {
				_, ok := connections[connect.RoomId]
				if ok {
					var message Message
					unmarshalError := json.Unmarshal([]byte(msgAsString), &message)
					if unmarshalError == nil {
						message.MessageId = xid.New().String()
						message.RoomId = connect.RoomId
						message.Timestamp = time.Now()
						saveMessage(&message)
					} else {
						fmt.Printf("\nError: %s", unmarshalError)
					}
				}
			}
		}
	})

	r.HandleFunc("/admin/{token}/settoken", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			return
		}
		vars := mux.Vars(r)
		adminToken = vars["token"]
	}).Methods("POST", "OPTIONS")

	r.HandleFunc("/admin/{token}/info", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		var token = vars["token"]
		if token == adminToken {
			fmt.Fprintf(w, "Connections: %d\n", len(connections))
			fmt.Fprintf(w, "Rooms: %d\n", len(inmemory))
			for roomId, messages := range inmemory {
				fmt.Fprintf(w, "  %s, %d\n", roomId, len(messages))
			}
		}
	}).Methods("GET")

	r.HandleFunc("/admin/{token}/reset", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		var token = vars["token"]
		if token == adminToken {
			inmemory = make(map[string][]Message)
		}
	}).Methods("POST")

	r.HandleFunc("/room/{roomId}/messages", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			return
		}
		vars := mux.Vars(r)
		roomId := vars["roomId"]
		if !checkRoomId(roomId) {
			http.Error(w, "{\"error\": \"RoomId invalid\"}", http.StatusBadRequest)
			return
		}

		var message Message

		json.NewDecoder(r.Body).Decode(&message)
		if message.Sender == "" {
			http.Error(w, "{\"error\": \"Sender invalid. Please provide a 'sender' property.\"}", http.StatusBadRequest)
			return
		}
		if message.Text == "" && message.Data == "" {
			http.Error(w, "{\"error\": \"Payload invalid. Please provide a 'text' property and/or 'data' property.\"}", http.StatusBadRequest)
			return
		}
		message.MessageId = xid.New().String()
		message.RoomId = roomId
		message.Timestamp = time.Now()
		saveMessage(&message)
		w.Header().Set("Content-Type", "applicatioon/json")
		json.NewEncoder(w).Encode(message)
	}).Methods("POST", "OPTIONS")

	r.HandleFunc("/room/{roomId}/messages", func(w http.ResponseWriter, r *http.Request) {

		if r.Method == http.MethodOptions {
			return
		}
		vars := mux.Vars(r)
		roomId := vars["roomId"]
		if !checkRoomId(roomId) {
			http.Error(w, "{\"error\": \"RoomId invalid\"}", http.StatusBadRequest)
			return
		}
		
		list, ok := inmemory[roomId]
		if ok {
			w.Header().Set("Content-Type", "applicatioon/json")
			json.NewEncoder(w).Encode(list)
		} else {
			http.Error(w, "{\"error\": \"Room not found\"}", http.StatusNotFound)
		}
	}).Methods("GET")

	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./public/")))

	http.ListenAndServe("127.0.0.1:28080", r)
}
