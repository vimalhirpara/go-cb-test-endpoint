package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type requestModel struct {
	MachineName string
	UserName    string
	TimeStamp   time.Time
}

var _requestModel requestModel

type responseModel struct {
	ResponseCode string    `json:"response-code"`
	TimeStamp    time.Time `json:"time-stamp,omitempty"`
	Body         string    `json:"body,omitempty"`
}

type AuthProfile struct {
	Token          string
	PrivateKeyPath string
}

var authProfile AuthProfile

const cbUrl string = "https://institution-api-sim.clearbank.co.uk/v1/test"

func main() {

	authProfile = AuthProfile{
		Token:          "M2FlMjM3MjFlZjJiNDc0ZTlkZmJkM2ZjZmVmYTI2NjU=.eyJpbnN0aXR1dGlvbklkIjoiNmJhZmEwNjItZWI1MC00YmRlLWI2MWYtN2JmOGVkYTZhYzlmIiwibmFtZSI6IkJldGEyMDIxVXB0bzIwMjQiLCJ0b2tlbiI6IjVDMzE5NkJCRjBERDQ0NjdCMkM0NUY0M0ZGQjY4NjczNTU5NzQ1MTI3RkQzNDg5MjkxODNBOUQ2RkFGMDVCQzIyNDcxRkQ0MkVDN0E0NTM3QURCMjBDMEYxQTE5NTlBQyJ9",
		PrivateKeyPath: "GoClearBank.prv"}

	r := Router()
	fmt.Println("Listening port 3000")
	log.Fatal(http.ListenAndServe(":3000", r))
}

func Router() *mux.Router {
	router := mux.NewRouter()

	router.HandleFunc("/", welcome).Methods("GET")
	router.HandleFunc("/healthcheck", healthcheck).Methods("GET")
	router.HandleFunc("/v1get", v1Get).Methods("GET")
	router.HandleFunc("/v1post", v1Post).Methods("POST")

	return router
}

func welcome(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Welcome to Clear Bank test environment."))
}

func healthcheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(getModel())
}

func v1Get(w http.ResponseWriter, r *http.Request) {

	request, err := http.NewRequest("GET", cbUrl, nil)
	if err != nil {
		json.NewEncoder(w).Encode(err)
	}

	id := uuid.New()
	request.Header.Add("X-Request-Id", id.String())
	request.Header.Add("Authorization", "Bearer "+authProfile.Token)

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		json.NewEncoder(w).Encode(err)
		return
	}

	defer response.Body.Close()

	_resp := responseModel{ResponseCode: response.Status, TimeStamp: time.Now()}
	json.NewEncoder(w).Encode(_resp)
}

func v1Post(w http.ResponseWriter, r *http.Request) {

	if r.Body == nil {
		json.NewEncoder(w).Encode("Please send some data.")
		return
	}

	// Get Body
	_ = json.NewDecoder(r.Body).Decode(&_requestModel)

	// Get Header
	dgtalSignature := r.Header.Get("DigitalSignature")
	contentType := r.Header.Get("Content-Type")
	postmanToken := r.Header.Get("Postman-Token")

	apiRequestText, err := json.Marshal(requestModel{MachineName: _requestModel.MachineName, UserName: _requestModel.UserName, TimeStamp: _requestModel.TimeStamp})
	if err != nil {
		json.NewEncoder(w).Encode(err)
		return
	}

	fmt.Println(string(apiRequestText))

	requestBody := bytes.NewBuffer(apiRequestText)

	request, err := http.NewRequest("POST", cbUrl, requestBody)
	if err != nil {
		json.NewEncoder(w).Encode(err)
		return
	}

	//id := uuid.New()
	request.Header.Add("X-Request-Id", postmanToken)
	request.Header.Add("Content-Type", contentType)
	request.Header.Add("DigitalSignature", dgtalSignature)
	request.Header.Add("Authorization", "Bearer "+authProfile.Token)

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		json.NewEncoder(w).Encode(err)
		return
	}

	defer response.Body.Close()

	bytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		json.NewEncoder(w).Encode(err)
		return
	}

	_resp := responseModel{ResponseCode: response.Status, TimeStamp: time.Now(), Body: string(bytes)}
	json.NewEncoder(w).Encode(_resp)
}

func getModel() requestModel {
	return requestModel{MachineName: "CON-IND-LPT47", UserName: "Vimal Hirapara", TimeStamp: time.Now()}
}
