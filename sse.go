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

// writeData fonksiyonu, belirtilen http.ResponseWriter üzerine sse veri yazmaktan sorumludur.
// Ayrıca, sseData.Connections altında tutulan bağlantılardan gelen mesajları okuyarak veriyi yazmaktadır.
// Fonksiyon, yazılan verinin boyutunu ve bir hata durumunda hata mesajını döndürmektedir.

// The writeData function is responsible for writing SSE data to the specified http.ResponseWriter.
// Additionally, it reads messages from connections stored under sseData.Connections to write the data.
// The function returns the size of the written data and an error message in case of an error.

func writeData(w http.ResponseWriter) (int, error) {
	return fmt.Fprintf(w, "data: %s\n\n", <-sseData.Connections[w].Message)
}

// sseStream fonksiyonu, bir http.HandlerFunc döndüren bir fonksiyondur.
// Bu handler fonksiyonu, Server-Sent Events (SSE) akışı için gerekli olan işlemleri gerçekleştirmektedir.
// SSE akışı başlatıldığında, connection'ın header'ını hazırlar, message kanalı oluşturur ve bağlantıyı sseData.Connections altında saklar.
// Fonksiyon, bir sonsöz bloğu içinde bağlantıyı temizler ve mesaj kanalını kapatır.
// Daha sonra bir sonsöz bloğu içinde sonsuz bir döngü başlatır, veri yazma işlemini gerçekleştirir ve flusher'ı kullanarak veriyi gönderir.

// The sseStream function returns an http.HandlerFunc.
// This handler function performs the necessary operations for Server-Sent Events (SSE) stream.
// When the SSE stream is initiated, it prepares the header for the connection, creates a message channel, and stores the connection under sseData.Connections.
// The function cleans up the connection and closes the message channel within a defer block.
// It then starts an infinite loop within another defer block to perform data writing and send the data using the flusher.

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

// sseMessage fonksiyonu, belirli bir mesajı Server-Sent Events (SSE) bağlantısına gönderen bir http.HandlerFunc döndürmektedir.
// Verilen mesajı alarak, bağlantıyı kilitleyip sseData.Connections altında mesajı gönderir.

// The sseMessage function returns an http.HandlerFunc that sends a specific message to a Server-Sent Events (SSE) connection.
// It locks and sends the message under sseData.Connections by extracting the given message.

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

// HTTP sunucusunu başlatır ve belirli HTTP URL'lerine yönlendirilen requestleri ele alacak handler fonksiyonları belirtir.
// "/stream" URL'sine gelen istekler için sseStream() handler fonksiyonunu, "/send" URL'sine gelen istekler için belirli bir mesajı gönderecek sseMessage() handler fonksiyonunu belirtir.
// Sonrasında HTTP sunucusunu belirtilen adres ve port üzerinden dinlemeye başlar.

// The main function is the main entry point of the program.
// It starts an HTTP server and specifies handler functions to handle requests directed to specific HTTP URLs.
// It associates the sseStream() handler function with requests to the "/stream" URL, and sseMessage() handler function with requests to the "/send", "/right", and "/left" URLs.
// Finally, it starts the HTTP server to listen on the specified address and port.

func main() {
	http.HandleFunc("/stream", sseStream())
	http.HandleFunc("/send", sseMessage("you can put json in here as the data"))
	http.HandleFunc("/right", sseMessage("right"))
	http.HandleFunc("/left", sseMessage("left"))
	log.Fatal("HTTP server error:", http.ListenAndServe("localhost:8080", nil))
}
