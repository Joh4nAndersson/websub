package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"encoding/json"
)

// stores topic, callback url and secret
type subscriberStorage struct {
	sync.Mutex
	subs map[string]map[string]string
}

func (ss *subscriberStorage) verifyIntent(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	r.Body.Close()
	req, err := http.NewRequest("GET", r.FormValue("hub.callback"), nil)
	if err != nil {
		log.Println(err)
	}

	// Generate challenge from callback url
	callback := r.FormValue("hub.callback")
	h := sha256.New()
	h.Write([]byte(callback))
	challenge := string(h.Sum(nil))

	// create query url
	query := req.URL.Query()
	topic := r.FormValue("hub.topic")
	secret := r.FormValue("hub.secret")
	mode := r.FormValue("hub.mode")
	query.Add("hub.mode", mode)
	query.Add("hub.topic", topic)
	query.Add("hub.challenge", challenge)
	req.URL.RawQuery = query.Encode()

	client := http.Client{}
	res, _ := client.Do(req)
	if res != nil {
		log.Println(res.StatusCode)
		body, _ := ioutil.ReadAll(res.Body)
		res.Body.Close()
		if strings.Compare(string(body), challenge) == 0 { // validation challenge success
			w.WriteHeader(http.StatusOK)
			ss.Lock()
			defer ss.Unlock()
			if strings.Compare(mode, "subscribe") == 0 {
				ss.subs[topic] = make(map[string]string)
				ss.subs[topic][callback] = secret
			} else {
				delete(ss.subs[topic], callback) // unsubscribe
			}

		} else { // validation challenge failed
			w.WriteHeader(http.StatusForbidden)
		}
		fmt.Fprint(w, nil)
	}

}

// sends some bytes to all subscribers
func (ss *subscriberStorage) jsonEndpoint(w http.ResponseWriter, r *http.Request) {
	data := []byte{1,2,3,4,5,6,7,8,9}
	j, _ := json.Marshal(data)
	// push to all subscribers
	r.ParseForm()
	postUpdatesToAll(j, r.FormValue("hub.topic"), ss)
}

func main(){
	ss := subscriberStorage{
		Mutex: sync.Mutex{},
		subs: make(map[string]map[string]string),
	}
	http.HandleFunc("/", ss.verifyIntent)
	http.HandleFunc("/publish", ss.jsonEndpoint)
	http.ListenAndServe(":8080", nil)
}

func generateHMAC(message string, key string) string{
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(message))
	algorithm := "sha256="
	encodedHMAC := hex.EncodeToString(h.Sum(nil))
	return algorithm + encodedHMAC
}

// posts to every subscriber (used to simulate a publisher)
func postUpdatesToAll(content []byte, topic string, ss *subscriberStorage) {
	body := ioutil.NopCloser(bytes.NewBuffer(content))
	ss.Lock()
	defer ss.Unlock()
	for _, subscriber := range ss.subs {
		for callback, secret := range subscriber {
			req, _ := http.NewRequest("POST", callback, body)
			req.Header.Set("Content-type", "application/json")
			req.Header.Set("X-Hub-Signature", generateHMAC(string(content), secret))
			req.Header.Add("Content-Length", strconv.Itoa(len(content)))

			client := http.Client{}
			res, _ := client.Do(req)
			if res != nil {
				log.Println(res.StatusCode)
				res.Body.Close()
			}
		}
	}
}

// post updates to everyone subscribed to a certain topic
func postUpdates(content []byte, topic string, ss *subscriberStorage) {
	body := ioutil.NopCloser(bytes.NewBuffer(content))
	ss.Lock()
	defer ss.Unlock()
	t := ss.subs[topic]
	for callback, secret := range t {
		req, _ := http.NewRequest("POST", callback, body)
		req.Header.Set("Content-type", "application/json")
		req.Header.Set("X-Hub-Signature", generateHMAC(string(content), secret))
		req.Header.Add("Content-Length", strconv.Itoa(len(content)))

		client := http.Client{}
		res, _ := client.Do(req)
		if res != nil {
			log.Println(res.StatusCode)
			res.Body.Close()
		}
	}
}

func (ss *subscriberStorage) receiveUpdatesFromPublisher(w http.ResponseWriter, r http.Request) {
	// no publisher available
	// should call postUpdates
}








