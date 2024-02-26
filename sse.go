package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"
)

// `SSEConnection` adında yeni bir veri türü tanımlanmıştır. 
// Bu veri türü, bir SSE bağlantısını temsil eder. 
// `Connection` alanı, bir `http.ResponseWriter` türünde bir bağlantıyı, 
// `Message` alanı ise mesajların iletilmesi için kullanılan bir kanalı içerir.

// This code defines a new data type named `SSEConnection`. 
// This data type represents an SSE connection. 
// It has two fields:`Connection`, which represents a connection of type `http.ResponseWriter`, 
// and `Message`, a channel used for sending messages.
type SSEConnection struct {
	Connection http.ResponseWriter
	Message    chan string
}

// sseStreamData struct holds the SSE connections and their corresponding message channels
type sseStreamData struct {
	Connections map[http.ResponseWriter]SSEConnection
	sync.RWMutex
}

var sseData = sseStreamData{
	Connections: make(map[http.ResponseWriter]SSEConnection),
}

func prepareHeaderForSSE(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
}

func writeData(w http.ResponseWriter) (int, error) {
	return fmt.Fprintf(w, "data: %s\n\n", <-sseData.Connections[w].Message)
}

func sseStream() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		prepareHeaderForSSE(w)

		msgChan := make(chan string)
		sseData.Lock()
		sseData.Connections[w] = SSEConnection{
			Connection: w,
			Message:    msgChan,
		}
		sseData.Unlock()

		defer func() {
			sseData.Lock()
			delete(sseData.Connections, w)
			close(msgChan)
			sseData.Unlock()
		}()

		flusher, _ := w.(http.Flusher)

		for {
			write, err := writeData(w)
			if err != nil {
				log.Println(err)
			}
			log.Println(write)
			flusher.Flush()
		}
	}
}

func sseMessage(message string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sseData.RLock()
		sc, ok := sseData.Connections[w]
		sseData.RUnlock()

		if ok {
			sc.Message <- message
		}
	}
}

func main() {
	http.HandleFunc("/stream", sseStream())
	http.HandleFunc("/send", sseMessage("you can put json in here as the data"))
	http.HandleFunc("/right", sseMessage("right"))
	http.HandleFunc("/left", sseMessage("left"))
	log.Fatal("HTTP server error:", http.ListenAndServe("localhost:8080", nil))
}
