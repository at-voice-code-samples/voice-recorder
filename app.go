package main

import (
	"os"
	"fmt"
	"net/http"
	"strconv"
	"encoding/json"
	"io"
	"github.com/go-redis/redis"
)

var downloadsDir = "/tmp/"

type userInfo struct {
	Registered bool
	FileName string
}

var redisClient *redis.Client

func createSession (callerNumber string) {
	info := userInfo{Registered: true, FileName: ""}
	jsonInfo,err := json.Marshal(info)
	err = redisClient.Set(callerNumber, string(jsonInfo), 0).Err()
	if err != nil {
		panic(err)
	}
}

func getRecording (callerNumber string) (bool, string) {
	info := userInfo{Registered: false, FileName: ""}
	jsonInfo, err := redisClient.Get(callerNumber).Result()
	if err == redis.Nil {
		return info.Registered, info.FileName
	}else{
		err = json.Unmarshal([]byte(jsonInfo), &info)
		if err != nil {
			panic(err)
		}
		return info.Registered, info.FileName
	}
}

func setRecording (callerNumber string, FileName string) {
	info := userInfo{Registered: true, FileName: FileName}
	jsonInfo,_ := json.Marshal(info)
	err := redisClient.Set(callerNumber, jsonInfo, 0).Err()
	if err != nil {
		panic(err)
	}
}

func main () {
	// Check command-line arguments
	if len(os.Args) != 3 {
		fmt.Println("Usage: ./app <callback-url> <port>")
		os.Exit(0)
	}
	
	// Connect to Redis
	redisClient = redis.NewClient(&redis.Options{
		Addr:     "10.0.1.29:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	err := redisClient.Set("key", "value", 0).Err()
	val,err := redisClient.Get("key").Result()
	if err != nil && val != "value" {
		panic(err)
	}

	recordingsHandler := http.FileServer(http.Dir("/tmp"))
	http.Handle("/recordings/", http.StripPrefix("/recordings/", recordingsHandler))

	http.HandleFunc("/fetch", func(w http.ResponseWriter, r *http.Request) {
		callerNumber := r.FormValue("callerNumber")
		sessionId := r.FormValue("sessionId")
		recordingUrl := r.FormValue("recordingUrl")
		
		if recordingUrl == "" {
			return
		}

		resp,err := http.Get(recordingUrl)
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()
		
		// Create file named after the session id
		FileName := sessionId+".mp3"
		out, err := os.Create(downloadsDir+FileName)
		if err != nil {
			panic(err)
		}
		defer out.Close()

		go io.Copy(out, resp.Body)

		setRecording(callerNumber, FileName)
	})
	
	http.HandleFunc("/digits", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		callerNumber := r.Form.Get("callerNumber")
		input,_ := strconv.ParseInt(r.Form.Get("dtmfDigits"), 10, 8)
		Registered,recording := getRecording(callerNumber)
		if Registered {
			switch input {
			case 1:
				fmt.Fprintf(w, `<Response>
						  <Record finishOnKey="#" trimSilence="true" playBeep="true" callbackUrl="%s">
						    <Say>Please record your message after the beep and press hash when done</Say>
						  </Record>
						</Response>`, os.Args[1]+"/fetch")
			case 2:
				if len(recording) == 0 {
					recording = "https://s3.eu-west-2.amazonaws.com/at-voice-sample/play.mp3"
				}else{
					recording = os.Args[1]+"/recordings/"+recording
				}
				fmt.Fprintf(w, `<Response>
						  <Play url="%s"/>
						</Response>`, recording)
			case 3:
				fmt.Fprintf(w, `<Response>
						  <Play url="https://s3.eu-west-2.amazonaws.com/at-voice-sample/play.mp3"/>
						</Response>`)
			case 4:
				fmt.Fprintf(w, `<Response><Say>Bye for now.</Say></Response>`)
			default:
				fmt.Fprintf(w, `<Response>
						  <GetDigits timeout='30' numDigits='1' callbackUrl='%s'>
						    <Say>Invalid option. Press 1 to record a message. Press 2 to listen to the previous recording. Press 3 to play a random tune. Press 4 to exit.</Say>
						  </GetDigits>
						  <Say>We did not get your option. Good bye</Say>
						</Response>`, os.Args[1]+"/digits")
			}
		}else{
			switch input {
			case 1:
				createSession(callerNumber)
				fmt.Fprintf(w, `<Response>
						  <GetDigits timeout='30' numDigits='1' callbackUrl='%s'>
						    <Say>Press 1 to record a message. Press 2 to listen to the previous recording. Press 3 to play a random tune. Press 4 to exit.</Say>
						  </GetDigits>
						  <Say>We did not get your option. Good bye</Say>
						</Response>`, os.Args[1]+"/digits")
			case 2:
				fmt.Fprintf(w, `<Response>
						  <Say>Good bye for now</Say>
						</Response>`)
			default:
				fmt.Fprintf(w, `<Response>
						  <GetDigits timeout='30' numDigits='1' callbackUrl='%s'>
						    <Say>Invalid option. Press 1 to record a message. Press 2 to listen to the previous recording. Press 3 to play a random tune. Press 4 to exit.</Say>
						  </GetDigits>
						  <Say>We did not get your option. Good bye</Say>
						</Response>`, os.Args[1]+"/digits")
			}
		}
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		callerNumber := r.Form.Get("callerNumber")
		// Check whether the number is Registered
		if Registered,_ := getRecording(callerNumber); Registered {
			fmt.Fprintf(w, `<Response>
					  <GetDigits timeout='30' numDigits='1' callbackUrl='%s'>
					    <Say>Hello, press 1 to record a message. Press 2 to listen to the previous recording. Press 3 to play a random tune. Press 4 to exit.</Say>
					  </GetDigits>
					  <Say>We did not get your option. Good bye</Say>
					</Response>`, os.Args[1]+"/digits")
		}else{
			fmt.Fprintf(w, `<Response>
					  <GetDigits timeout='30' numDigits='1' callbackUrl='%s'>
					    <Say>Hello, press 1 to register. Press 2 to exit</Say>
					  </GetDigits>
					  <Say>We did not get your option. Good bye</Say>
					</Response>`, os.Args[1]+"/digits")
		}
	})
	
	http.ListenAndServe(":"+os.Args[2], nil)
}
