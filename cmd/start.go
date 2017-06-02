// Copyright © 2017 NAME HERE <EMAIL ADDRESS>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

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
	"sync"
	"time"

	resty "gopkg.in/resty.v0"

	"github.com/cpucycle/astrotime"
	"github.com/hashicorp/mdns"
	"github.com/spf13/cobra"
)

var (
	latitude  string
	longitude string
	followSun bool
	cycle     string
	directory string
	interval  int
	//LATITUDE = float64(39.7293)
	//LONGITUDE = float64(104.8673)
)

//Place to hold known connections so we're not running multiple getters
type Cameras struct {
	mutex sync.Mutex
	Hosts []string
}

var cameras Cameras

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start looking for cameras and saving images",
	Long: `Continue to check for new cameras starting
and image processor for each new one found.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("start called")
		startQueries()
	},
}

func init() {
	RootCmd.AddCommand(startCmd)
	startCmd.Flags().StringVarP(&latitude, "lat", "", "39.7293", "Your Latitude")
	startCmd.Flags().StringVarP(&longitude, "long", "", "104.8673", "Your Longitude")
	startCmd.Flags().BoolVarP(&followSun, "follow-sun", "", false, "Use to follow diurnal cycle")
	startCmd.Flags().StringVarP(&cycle, "cycle", "", "day", "Set to either day/night")
	startCmd.Flags().StringVarP(&directory, "directory", "", "shots/", "Set the directory to save pictures")
	startCmd.Flags().IntVarP(&interval, "interval", "", 2, "Set the interval to capture in minutes")
	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// startCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// startCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func startQueries() {
	ticker := time.NewTicker(time.Millisecond * 500)
	quit := make(chan struct{})
	go func() {
		// Make a channel for results and start listening
		entriesCh := make(chan *mdns.ServiceEntry, 4)
		for {
			select {
			case <-ticker.C:
				mdns.Lookup("_goshot._tcp", entriesCh)
				for entry := range entriesCh {
					if !findCamera(entry.AddrV4.String()) {
						log.Println("No camera found at that address: ", entry.AddrV4.String())
						startCapture(entry)
					}
				}
			case <-quit:
				ticker.Stop()
				return
			}
		}

	}()
}

func startCapture(entry *mdns.ServiceEntry) {

	//go queryServer(&entriesCh)

	fmt.Printf("Got new entry: %v\n", entry)

	ticker := time.NewTicker(time.Duration(interval*60) * time.Second)
	quit := make(chan struct{})
	addr := entry.AddrV4.String()
	addCamera(addr)
	defer removeCamera(addr)
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
	//log.Println(imageString)
	reader := base64.NewDecoder(base64.StdEncoding, strings.NewReader(imageString))
	image, _, err := image.Decode(reader)
	if err != nil {
		log.Println(err)
		return
	}

	err = os.MkdirAll(directory, 0777)
	out, err := os.Create(directory + "/" + strconv.FormatInt(makeTimestamp(), 10) + ".jpg")

	if err != nil {
		log.Println(err)
		return
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
	lat, _ := strconv.ParseFloat(latitude, 64)
	long, _ := strconv.ParseFloat(longitude, 64)
	sunup := astrotime.CalcSunrise(now, lat, long)
	log.Println("Sunup: " + sunup.Format("2006-01-02 15:04:05"))
	sundown := astrotime.CalcSunset(now, lat, long)
	log.Println("Sundown: " + sundown.Format("2006-01-02 15:04:05"))
	if now.Unix() > (sunup.Unix()+1800) && now.Unix() < (sundown.Unix()-1800) {
		log.Println("It's daylight y'all!")
		return true
	}
	log.Println("It's dark y'all!")
	return false
}

func findCamera(host string) bool {
	cameras.mutex.Lock()
	defer cameras.mutex.Unlock()
	found := false
	for i := range cameras.Hosts {
		if cameras.Hosts[i] == host {
			found = true
		}
	}
	return found
}

func addCamera(host string) {
	cameras.mutex.Lock()
	defer cameras.mutex.Unlock()
	cameras.Hosts = append(cameras.Hosts, host)
}

func removeCamera(host string) {
	cameras.mutex.Lock()
	defer cameras.mutex.Unlock()
	for key, value := range cameras.Hosts {
		if value == host {
			cameras.Hosts = append(cameras.Hosts[:key], cameras.Hosts[key+1:]...)
		}
	}

}
