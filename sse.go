package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"
)


// Bu veri türü, bir SSE bağlantısını temsil eder. 
// `Connection` alanı, bir `http.ResponseWriter` türünde bir bağlantıyı, 
// `Message` alanı ise mesajların iletilmesi için kullanılan bir kanalı içerir.

// This data type represents an SSE connection. 
// It has two fields:`Connection`, which represents a connection of type `http.ResponseWriter`, 
// and `Message`, a channel used for sending messages.

type SSEConnection struct {
	Connection http.ResponseWriter
	Message    chan string
}


// Bu veri türü, SSE bağlantılarını yönetmek için kullanılır. 
// `Connections` adında bir harita (map) içerir, bu harita her bir `http.ResponseWriter` bağlantısına karşılık gelen bir `SSEConnection` nesnesi tutar. 
// Ayrıca, veriyi eşzamanlı bir şekilde güvenli bir şekilde erişmek için `sync.RWMutex` türünde bir kilit içerir.

// This data type is used to manage SSE connections. 
// It contains a map named `Connections` which maps each `http.ResponseWriter` connection to an `SSEConnection` object. 
// Additionally, it includes a `sync.RWMutex` lock for safely accessing the data concurrently.

type sseStreamData struct {
	Connections map[http.ResponseWriter]SSEConnection
	sync.RWMutex
}


// Bu değişken, SSE bağlantılarını yönetmek için kullanılan veri yapısını temsil eder. 
// `Connections` adında bir harita (map) içerir ve bu harita, her bir `http.ResponseWriter` bağlantısına karşılık gelen bir `SSEConnection` nesnesini depolar. 
// Değişken, bu haritayı oluşturmak için `make` fonksiyonu kullanılarak başlatılmıştır.

// This variable represents the data structure used to manage SSE connections. 
// It contains a map named `Connections`, which stores an `SSEConnection` object for each `http.ResponseWriter` connection. 
// The variable is initialized by using the `make` function to create this map.

var sseData = sseStreamData{
	Connections: make(map[http.ResponseWriter]SSEConnection),
}

// Bu fonksiyon, Server-Sent Events (SSE) için gerekli olan başlık bilgilerini hazırlar. 
// Fonksiyon, `http.ResponseWriter` arabirimine sahip bir parametre alır ve bu parametre üzerinden HTTP yanıtının başlığını yapılandırır. 
// İlgili başlıklar şunlardır: `Content-Type` (içerik türü), `Cache-Control` (önbellek kontrolü), `Connection` (bağlantı) ve `Access-Control-Allow-Origin` (kök başlığa izin verilen kaynaklar).

// This function prepares the necessary header information for Server-Sent Events (SSE).
// The function takes a parameter of type `http.ResponseWriter` and configures the headers of the HTTP response using this parameter.
// The relevant headers set in this function are: `Content-Type`, `Cache-Control`, `Connection`, and `Access-Control-Allow-Origin`.

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
