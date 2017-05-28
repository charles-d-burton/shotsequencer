package main

import (
	"fmt"
	"github.com/hashicorp/mdns"
	"gopkg.in/resty.v0"
	"strconv"
	"time"
)

func main() {
	// Make a channel for results and start listening
	entriesCh := make(chan *mdns.ServiceEntry, 4)
	mdns.Lookup("_goshot._tcp", entriesCh)

	//go func() {
	for entry := range entriesCh {
		fmt.Printf("Got new entry: %v\n", entry)
		ticker := time.NewTicker(30 * time.Second)
		quit := make(chan struct{})
		go func() {
			for {
				select {
				case <-ticker.C:
					callCamera(entry.AddrV4.String(), strconv.Itoa(entry.Port))
				case <-quit:
					ticker.Stop()
					return
				}
			}
		}()
	}
	//}()

	// Start the lookup
	//close(entriesCh)
}

func callCamera(server, port string) {
	resp, err := resty.R().Get("http://" + server + ":" + port + "/shot")

	// explore response object
	fmt.Printf("\nError: %v", err)
	fmt.Printf("\nResponse Status Code: %v", resp.StatusCode())
	fmt.Printf("\nResponse Status: %v", resp.Status())
	fmt.Printf("\nResponse Time: %v", resp.Time())
	fmt.Printf("\nResponse Recevied At: %v", resp.ReceivedAt())
	fmt.Printf("\nResponse Body: %v", resp) // or resp.String() or string(resp.Body())
}
