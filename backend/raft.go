package main

import (
	"encoding/json"
	"io"
	"log"
	"math/rand"
	"net"
	"time"
)

// Global variables
var LeaderNode = -1
var votes chan Request

func sendVoteRequest(request *Request, nodeAddr string) {
	responseChan := make(chan Request)
	requestBytes, marshalErr := json.Marshal(request)
	if marshalErr != nil {
		log.Print("Error writing marshalling: ")
		log.Fatal(marshalErr)
	}

	conn, err := net.Dial("tcp", nodeAddr)
	if err != nil && err != io.EOF {
		log.Println("After dial:", err)
		responseChan <- Request{Vote: -1}
	}

	log.Print("Sending: ")
	log.Print(string(requestBytes))
	conn.Write(requestBytes)
	defer conn.Close()
}

func requestVote(nodeAddr string, votes chan Request) {
	req := Request{
		Action:   4,
		SenderID: currentNodeID,
	}
	sendVoteRequest(&req, nodeAddr)
}

func requestVotes(backends []string) {
	for _, nodeAddr := range backends {
		go requestVote(nodeAddr, votes)
	}

}

func countVotes(done chan int, backends []string) {
	numNodes := len(backends)
	numQuorum := (numNodes / 2) + 1

	voteMap := make(map[int]int)
	voteMap[currentNodeID] = 1

	for i := 0; i < numNodes; i++ {
		log.Println("WAITING FOR VOTES!")
		voteReq := <-votes

		_, nodeInMap := voteMap[voteReq.Vote]
		log.Println("Received vote for: ", voteReq.Vote)
		if nodeInMap {
			voteMap[voteReq.Vote]++
		} else {
			voteMap[voteReq.Vote] = 1
		}

		// Check if the node that was voted has received a quorum of votes
		if voteMap[voteReq.Vote] > numQuorum {
			log.Println("QUORUM! Leader is ", LeaderNode)
			LeaderNode := voteReq.Vote
			log.Println("Leader is: ", LeaderNode)
			done <- LeaderNode
			return
		}
	}
}

func startElection(backends []string) {
	done := make(chan int)
	go countVotes(done, backends)
	randTimeout := time.Duration((rand.Float64() * 150) + 150)
	time.Sleep(randTimeout * time.Millisecond)

	for {
		select {
		case <-done:
			return
		case <-time.After(time.Duration(1600) * time.Millisecond):
			log.Println("repeat election")
			go requestVotes(backends)
		}
	}

}
