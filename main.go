package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cpucycle/astrotime"
	"github.com/hashicorp/mdns"
	"gopkg.in/resty.v0"
)

const LATITUDE = float64(39.7293)
const LONGITUDE = float64(104.8673)

func main() {
	// Make a channel for results and start listening
	entriesCh := make(chan *mdns.ServiceEntry, 4)
	mdns.Lookup("_goshot._tcp", entriesCh)
	//go queryServer(&entriesCh)
	for entry := range entriesCh {
		fmt.Printf("Got new entry: %v\n", entry)

		ticker := time.NewTicker(60 * time.Second)
		quit := make(chan struct{})
		addr := entry.AddrV4.String()
		port := strconv.Itoa(entry.Port)
		message, err := callCamera(addr, port)
		if err != nil {
			log.Println(err)
			//ticker.Stop()
		}
		//log.Println(message)
		var image Image
		err = json.Unmarshal([]byte(message), &image)
		saveImage(image.Image)
		go func(addr string, port string) {
			for {
				select {
				case <-ticker.C:
					if isDaylight() {
						message, err := callCamera(addr, port)
						if err != nil {
							log.Println(err)
							return
							//ticker.Stop()
						}
						//log.Println(message)
						var image Image
						err = json.Unmarshal([]byte(message), &image)
						if image.Error != "" {
							log.Println(image.Error)
							return
						}
						saveImage(image.Image)
					}
				case <-quit:
					ticker.Stop()
					return
				}
			}
		}(addr, port)
	}

	//close(entriesCh)
}

func callCamera(server, port string) (string, error) {
	resp, err := resty.R().Get("http://" + server + ":" + port + "/shot")

	// explore response object
	log.Println("\nError: ", err)
	log.Println("\nResponse Status Code: ", resp.StatusCode())
	log.Println("\nResponse Status:", resp.Status())
	log.Println("\nResponse Time:", resp.Time())
	log.Println("\nResponse Recevied At: ", resp.ReceivedAt())
	return string(resp.Body()), err
}

type Image struct {
	Error string `json:"error"`
	Image string `json:"image"`
}

func saveImage(imageString string) {

	reader := base64.NewDecoder(base64.StdEncoding, strings.NewReader(imageString))
	image, _, err := image.Decode(reader)
	if err != nil {
		log.Println(err)
	}

	err = os.MkdirAll("pics/", 0777)
	out, err := os.Create("pics/" + strconv.FormatInt(makeTimestamp(), 10) + ".jpg")

	if err != nil {
		log.Println(err)
	}

	var opt jpeg.Options
	opt.Quality = 90
	err = jpeg.Encode(out, image, &opt)
	if err != nil {
		log.Println(err)
	}
}

func makeTimestamp() int64 {
	return time.Now().UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))
}

func isDaylight() bool {
	now := time.Now()
	sunup := astrotime.CalcSunrise(now, LATITUDE, LONGITUDE)
	log.Println("Sunup: " + sunup.Format("2006-01-02 15:04:05"))
	sundown := astrotime.CalcSunset(now, LATITUDE, LONGITUDE)
	log.Println("Sundown: " + sundown.Format("2006-01-02 15:04:05"))
	if now.Unix() > (sunup.Unix()+1800) && now.Unix() < (sundown.Unix()-1800) {
		log.Println("It's daylight y'all!")
		return true
	}
	log.Println("It's dark y'all!")
	return false
}
