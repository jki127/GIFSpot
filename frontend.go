package main

import (
	"encoding/json"
	"flag"
	"github.com/kataras/iris"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"time"
)

type Gif struct {
	Name        string
	Description string
	Url         string
	Deleted     bool
	Index       *uint64
}

// Request
// Action Mappings
// -1: PING-ACK
// 0 : INDEX
// 1 : CREATE
// 2 : UPDATE
// 3 : DESTROY
type Request struct {
	Action int
	Data   string
	Index  int
}

var gifs []Gif
var backendFlag = flag.String("backend", ":8090",
	"Backend addresses to connect to (comma-separated, no spaces)")
var portNum = flag.Int("listen", 8080, "An int specifying a custom port")
var backends []string

// failureDetector runs a loop which pings the backend server every 5 seconds
// to check if it is still responding. If it not longer responding, it will
// print a message to stdout
func failureDetector(backend string) {
	request := Request{Action: -1}

	for {
		time.Sleep(5 * time.Second)
		conn, err := net.Dial("tcp", backend)
		if err != nil && err != io.EOF {
			log.Println("Detected Failure on", backend)
		} else {
			requestBytes, marshalErr := json.Marshal(&request)
			if marshalErr != nil {
				log.Print("Error writing marshalling: ")
				log.Fatal(marshalErr)
			}
			conn.Write(requestBytes)
		}

	}
}

// parseGifsJson takes in a JSON string representing an array of GIF Objects
// and returns an array of GIFs
func parseGifsJson(gifsJson string) []Gif {
	var parsedGifs []Gif
	dec := json.NewDecoder(strings.NewReader(gifsJson))
	// read open bracket
	_, err := dec.Token()
	if err != nil {
		log.Print("Error reading open bracket")
		log.Fatal(err)
	}
	// while the array contains values
	for dec.More() {
		var gif Gif
		err := dec.Decode(&gif)
		if err != nil {
			log.Print("Error parsing JSON body")
			log.Fatal(err)
		}
		parsedGifs = append(parsedGifs, gif)
	}
	// read closing bracket
	_, err = dec.Token()
	if err != nil {
		log.Print("Error reading closing bracket")
		log.Fatal(err)
	}
	return parsedGifs
}

// sendRequest sends a request to the backend and returns an array of all gifs
func sendRequest(request *Request) []Gif {
	requestBytes, marshalErr := json.Marshal(request)
	if marshalErr != nil {
		log.Print("Error writing marshalling: ")
		log.Fatal(marshalErr)
	}

	// TODO should send request to the leader
	// send the request to the first backend
	backend := backends[0]
	conn, err := net.Dial("tcp", backend)
	if err != nil && err != io.EOF {
		log.Print("After dial:")
		log.Fatal(err)
	}

	log.Print("Sending: ")
	log.Print(string(requestBytes))
	conn.Write(requestBytes)

	defer conn.Close()
	responseBytes := make([]byte, 90000)
	_, readErr := conn.Read(responseBytes)
	if readErr != nil && readErr != io.EOF {
		log.Print("After reading:")
		log.Fatal(err)
	}

	log.Print("Receiving: ")
	log.Print(string(responseBytes))
	return parseGifsJson(string(responseBytes))
}

// GIF Index page
// This page shows all the gifs that have been created, but not deleted
func gifIndexPage(ctx iris.Context) {
	initGifsRequest := Request{Action: 0}
	gifs = sendRequest(&initGifsRequest)

	var indexGifs []Gif

	// Filter out gifs that have been deleted
	for _, gif := range gifs {
		if !gif.Deleted {
			indexGifs = append(indexGifs, gif)
		}
	}

	ctx.ViewData("gifs", indexGifs)
	ctx.View("index.html")
}

// New GIF page
// This page gives the user a form to create a new GIF entry
func gifNewPage(ctx iris.Context) {
	ctx.View("new.html")
}

// GIF show page
// This page shows the information of an individial GIF entry
func gifShowPage(ctx iris.Context) {
	initGifsRequest := Request{Action: 0}
	gifs = sendRequest(&initGifsRequest)

	index, _ := ctx.Params().GetInt64("id")
	if !gifs[index].Deleted {
		ctx.ViewData("index", index)
		ctx.ViewData("name", gifs[index].Name)
		ctx.ViewData("description", gifs[index].Description)
		ctx.ViewData("url", gifs[index].Url)
		ctx.View("show.html")
	} else {
		ctx.WriteString("This GIF has been deleted")
	}
}

// GIF edit page
// This pages gives the user a form to edit a GIF entry
func gifEditPage(ctx iris.Context) {
	initGifsRequest := Request{Action: 0}
	gifs = sendRequest(&initGifsRequest)

	index, _ := ctx.Params().GetInt64("id")
	if !gifs[index].Deleted {
		ctx.ViewData("index", index)
		ctx.ViewData("name", gifs[index].Name)
		ctx.ViewData("description", gifs[index].Description)
		ctx.ViewData("url", gifs[index].Url)
		ctx.View("edit.html")
	} else {
		ctx.WriteString("This GIF has been deleted")
	}
}

// GIF delete page
// This page asks the user to confirm a delete action
func gifDeletePage(ctx iris.Context) {
	initGifsRequest := Request{Action: 0}
	gifs = sendRequest(&initGifsRequest)

	index, _ := ctx.Params().GetInt64("id")
	if !gifs[index].Deleted {
		ctx.ViewData("title", gifs[index].Name)
		ctx.ViewData("index", index)
		ctx.View("delete.html")
	} else {
		ctx.WriteString("This GIF has already been deleted")
	}
}

// Create GIF endpoint function
// This function takes in a POST request and adds a new GIF to the gifs array
func gifCreate(ctx iris.Context) {
	name := ctx.PostValue("name")
	description := ctx.PostValue("description")
	url := ctx.PostValue("url")

	newGif := Gif{Name: name, Description: description, Url: url}
	gifJson, _ := json.Marshal(newGif)
	createGifRequest := Request{Action: 1, Data: string(gifJson)}
	sendRequest(&createGifRequest)
	ctx.Redirect("/")
}

// Update GIF endpoint function
// This function takes in a POST request and updates the information of a
// single GIF entry
func gifUpdate(ctx iris.Context) {
	initGifsRequest := Request{Action: 0}
	gifs = sendRequest(&initGifsRequest)

	index, _ := ctx.Params().GetInt("id")
	gif := &gifs[index]
	gif.Name = ctx.PostValue("name")
	gif.Description = ctx.PostValue("description")
	gif.Url = ctx.PostValue("url")
	str_index := strconv.Itoa(index)

	gifJson, _ := json.Marshal(gif)
	updateGifRequest := Request{Action: 2, Data: string(gifJson), Index: index}
	gifs = sendRequest(&updateGifRequest)
	ctx.Redirect("/gif/" + str_index)
}

// GIF Destroy endpoint function
// This function takes in a POST request and deletes a GIF. The GIF is removed
// from the gifs array.
func gifDestroy(ctx iris.Context) {
	index, _ := ctx.Params().GetInt("id")
	destroyGifRequest := Request{Action: 3, Index: index}
	gifs = sendRequest(&destroyGifRequest)

	ctx.Redirect("/")
}

func main() {
	flag.Parse()
	backends = strings.Split(*backendFlag, ",")
	for _, backend := range backends {
		go failureDetector(backend)
	}
	app := iris.New()
	// Render HTML files and re-render when files are changed
	app.RegisterView(iris.HTML("./views", ".html").Reload(true))
	// Render files in /assets statically
	app.StaticWeb("/assets", "./assets")

	// ROUTES
	app.Handle("GET", "/", gifIndexPage)
	app.Handle("GET", "/gif/new", gifNewPage)
	app.Handle("GET", "/gif/{id:long}", gifShowPage)
	app.Handle("GET", "/gif/edit/{id:long}", gifEditPage)
	app.Handle("GET", "/gif/delete/{id:long}", gifDeletePage)
	app.Handle("POST", "/gif/create", gifCreate)
	app.Handle("POST", "/gif/update/{id:long}", gifUpdate)
	app.Handle("POST", "/gif/destroy/{id:long}", gifDestroy)

	// RETRIEVE FLAG FOR CUSTOM PORT
	port := ":" + strconv.Itoa(*portNum)
	app.Run(iris.Addr(port), iris.WithoutServerError(iris.ErrServerClosed))
}
