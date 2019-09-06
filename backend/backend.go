package main

import (
	"encoding/json"
	"flag"
	"io"
	"log"
	"net"
	"runtime"
	"strings"
	"sync/atomic"
)

type Gif struct {
	Name        string
	Description string
	Url         string
	Deleted     bool
	Index       uint64
}

// Delete sets the the Deleted attribute of gif to true
func (gif *Gif) Delete() {
	gif.Deleted = true
}

type Gifs struct {
	Data   []Gif
	done   *uint64
	ticket *uint64
}

// NewGifs initializes a Gifs object
func NewGifs() Gifs {
	g := Gifs{}
	g.done = new(uint64)
	g.ticket = new(uint64)
	return g
}

// Put adds a Gif to the datastore
// Put grabs a "ticket" which is an index that only this thread will get
// Then the done variable is updated to the new ticket value atomically
func (gifs *Gifs) Put(gif Gif) {
	t := atomic.AddUint64(gifs.ticket, 1) - 1
	// append an empty gif to expand the slice if needed
	gifs.Data = append(gifs.Data, Gif{Deleted: true})
	gif.Index = t
	gifs.Data[t] = gif
	for !atomic.CompareAndSwapUint64(gifs.done, t, t+1) {
		runtime.Gosched()
	}
}

// GetDone returns the part of the gif datastore that has been written
func (gifs *Gifs) GetDone() []Gif {
	return gifs.Data[:atomic.LoadUint64(gifs.done)]
}

// Request
// Action Mappings
// -1: PING-ACK
// 0 : INDEX
// 1 : CREATE
// 2 : UPDATE
// 3 : DESTROY
// 4 : RequestVote
type Request struct {
	Action   int
	Data     string
	Index    int
	Vote     int
	Term     int
	SenderID int
}

// Global variables
var gifs Gifs
var currentNodeID int

// parseRequestJson takes in a JSON string representing a request from the
// client and returns a Request object
func parseRequestJson(requestJson *string) Request {
	dec := json.NewDecoder(strings.NewReader(*requestJson))
	var request Request
	bodyErr := dec.Decode(&request)
	if bodyErr != nil && bodyErr != io.EOF {
		log.Print("Error parsing JSON body")
		log.Fatal(bodyErr)
	}
	return request
}

// parseGifJson takes in a JSON string representing a GIF and returns a GIF
// object
func parseGifJson(gifJson *string) Gif {
	dec := json.NewDecoder(strings.NewReader(*gifJson))
	var gif Gif
	bodyErr := dec.Decode(&gif)
	if bodyErr != nil && bodyErr != io.EOF {
		log.Print("Error parsing JSON body")
		log.Fatal(bodyErr)
	}
	return gif
}

// allGifsJson returns all GIFs in a byte array format
func allGifsJson() []byte {
	allGifsBytes, err := json.Marshal(gifs.GetDone())
	if err != nil && err != io.EOF {
		log.Print("Error running json.Marshal")
		log.Fatal(err)
	}
	return allGifsBytes
}

func encodeRequest(request *Request) []byte {
	requestBytes, err := json.Marshal(*request)
	if err != nil && err != io.EOF {
		log.Print("encodeRequest(): Error running json.Marshal")
		log.Fatal(err)
	}
	return requestBytes
}

// handleConnection
// This function reads data from a connection sent by a client
// The server will then perform some CRUD action on `gifs` specified in the
// request
func handleConnection(conn net.Conn) {
	defer conn.Close()
	clientAddr := conn.RemoteAddr().String()
	log.Print(clientAddr)

	bytes := make([]byte, 4096)
	_, err := conn.Read(bytes)
	if err != nil && err != io.EOF {
		log.Fatal(err)
	}

	requestJson := string(bytes)
	log.Print(requestJson)
	request := parseRequestJson(&requestJson)

	// PING-ACK
	if request.Action == -1 {
		conn.Close()
		return
	}

	log.Print("sending:")
	// INDEX
	if request.Action == 0 {
		jsonBytes := allGifsJson()
		conn.Write(jsonBytes)
		log.Print(string(jsonBytes))
	}

	// CREATE
	if request.Action == 1 {
		newGif := parseGifJson(&request.Data)
		gifs.Put(newGif)
		jsonBytes := allGifsJson()
		conn.Write(jsonBytes)
		log.Print(string(jsonBytes))
	}

	// UPDATE
	if request.Action == 2 {
		newGif := parseGifJson(&request.Data)
		gifs.Data[request.Index] = newGif

		jsonBytes := allGifsJson()
		conn.Write(jsonBytes)
		log.Print(string(jsonBytes))
	}

	// DESTROY
	if request.Action == 3 {
		index := request.Index
		gifs.Data[index].Delete()

		jsonBytes := allGifsJson()
		conn.Write(jsonBytes)
		log.Print(string(jsonBytes))
	}

	// VoteRequest
	if request.Action == 4 {
		response := Request{
			Action:   5,
			Vote:     LeaderNode,
			SenderID: currentNodeID,
		}
		if LeaderNode == -1 {
			response.Vote = request.SenderID
		}

		jsonBytes := encodeRequest(&request)
		conn.Write(jsonBytes)
		log.Print(string(jsonBytes))
	}

	// Vote
	if request.Action == 5 {
		votes <- request
	}
}

func parseResponse(response *string) Request {
	dec := json.NewDecoder(strings.NewReader(*response))
	var decoded Request
	bodyErr := dec.Decode(&decoded)
	if bodyErr != nil && bodyErr != io.EOF {
		log.Print("Error parsing JSON body")
		log.Fatal(bodyErr)
	}
	return decoded
}

func main() {
	gifs = NewGifs()
	gifs.Put(Gif{Name: "Keyboard Cat",
		Description: "A very cool cat typing on a computer keyboard",
		Url:         "https://media.giphy.com/media/o0vwzuFwCGAFO/giphy.gif"})

	gifs.Put(Gif{Name: "Olympics guy has two glasses!",
		Description: "Glasses under his glasses?!?!",
		Url:         "https://i.imgur.com/AGmFUDe.gif"})

	gifs.Put(Gif{Name: "Kevin Bacon",
		Description: "I don't know what he needs to be freed from",
		Url:         "https://media.giphy.com/media/k15qyyo5lhQRO/giphy.gif"})

	gifs.Put(Gif{Name: "Patrick fixes the computer",
		Description: "That's one way to fix a computer",
		Url:         "https://media.giphy.com/media/4no7ul3pa571e/giphy.gif"})

	host := flag.String("listen", "8090", "Host address to listen on")
	backendFlag := flag.String("backend", "",
		"Addresses of other backends (comma-separated, no spaces)")
	idFlag := flag.Int("id", -1, "A unique ID number for backend server")
	flag.Parse()

	currentNodeID = *idFlag

	if currentNodeID == -1 {
		log.Fatal("Must provide ID flag to start backend node")
	}

	if len(*backendFlag) > 0 {
		backends := strings.Split(*backendFlag, ",")
		votes = make(chan Request)
		go startElection(backends)
	}

	ln, err := net.Listen("tcp", "localhost:"+*host)
	if err != nil && err != io.EOF {
		log.Print("Can't bind to port")
		log.Fatal(err)
	}

	for {
		conn, err := ln.Accept()
		if err != nil && err != io.EOF {
			log.Fatal(err)
		}

		log.Print("conn accepted")
		go handleConnection(conn)
	}
}
