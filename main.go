package main

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	PublicKeyPath  string
}

var authProfile AuthProfile

const cbUrl string = "https://institution-api-sim.clearbank.co.uk/v1/test"

func main() {

	authProfile = AuthProfile{
		Token:          "OTMwYzM0ZDBkNmI5NDcxOTg2NDczODEzYTZhY2YxZDk=.eyJpbnN0aXR1dGlvbklkIjoiNmJhZmEwNjItZWI1MC00YmRlLWI2MWYtN2JmOGVkYTZhYzlmIiwibmFtZSI6IlRlc3QiLCJ0b2tlbiI6IkEwQTY4NzZDNEEwMDQ3MTBBNDlDREU0MTY1NTRDQkQ3MDUwREZDNzM5QTU1NDVBODk3MUVBMUE2Mzg1RkExRkU1QTYwMDE0NjU1NjY0NTYxQThDNTk3QUZGMDZFMEU4QiJ9",
		PrivateKeyPath: "GoClearBank.prv",
		PublicKeyPath:  "GoClearBank.pub"}

	r := Router()
	// fmt.Println("Listening port 3000")
	// log.Fatal(http.ListenAndServe(":3000", r))

	srv := &http.Server{
		Handler:      r,
		Addr:         ":3000",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Start Server
	go func() {
		log.Println("Starting Server")
		if err := srv.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()
	waitForShutdown(srv)
}

func waitForShutdown(srv *http.Server) {
	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// Block until we receive our signal.
	<-interruptChan

	// create a deadline to wait for.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	srv.Shutdown(ctx)

	log.Println("Shutting down")
	os.Exit(0)
}

// Handler

func Router() *mux.Router {
	router := mux.NewRouter()

	router.HandleFunc("/", welcome).Methods("GET")
	router.HandleFunc("/healthcheck", healthcheck).Methods("GET")
	router.HandleFunc("/v1get", v1Get).Methods("GET")
	router.HandleFunc("/v1post", v1Post).Methods("POST")

	return router
}

// Endpoints

func welcome(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Welcome to Clear Bank test environment."))
}

func healthcheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(getModel())
}

func v1Get(w http.ResponseWriter, r *http.Request) {

	// Get Header
	postmanToken := r.Header.Get("Postman-Token") // Get Unique ID / UUID

	request, err := http.NewRequest("GET", cbUrl, nil)
	if err != nil {
		json.NewEncoder(w).Encode(err)
	}

	request.Header.Add("X-Request-Id", postmanToken)
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
	//dgtalSignature := r.Header.Get("DigitalSignature")
	postmanToken := r.Header.Get("Postman-Token") // Get Unique ID / UUID

	//apiRequestText, err := json.Marshal(requestModel{MachineName: _requestModel.MachineName, UserName: _requestModel.UserName, TimeStamp: _requestModel.TimeStamp})
	apiRequestText, err := json.Marshal(r.Body)
	if err != nil {
		json.NewEncoder(w).Encode(err)
		return
	}

	privateKey, err := loadPrivateKey()
	if err != nil {
		json.NewEncoder(w).Encode(err)
		return
	}

	dgtalSignature, err := Generate(apiRequestText, privateKey)
	if err != nil {
		json.NewEncoder(w).Encode(err)
		return
	}

	requestBody := bytes.NewBuffer(apiRequestText)

	request, err := http.NewRequest("POST", cbUrl, requestBody)
	if err != nil {
		json.NewEncoder(w).Encode(err)
		return
	}

	request.Header.Add("X-Request-Id", postmanToken)
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("DigitalSignature", dgtalSignature)
	request.Header.Add("Authorization", "Bearer "+authProfile.Token)

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		json.NewEncoder(w).Encode(err)
		return
	}

	defer response.Body.Close()

	respBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		json.NewEncoder(w).Encode(err)
		return
	}
	_resp := responseModel{ResponseCode: response.Status, TimeStamp: time.Now(), Body: string(respBytes)}
	json.NewEncoder(w).Encode(_resp)
}

// Model

func getModel() requestModel {
	return requestModel{MachineName: "CON-IND-LPT47", UserName: "Vimal Hirapara", TimeStamp: time.Now()}
}

// Digital Signature

func Generate(text []byte, privateKey *rsa.PrivateKey) (string, error) {
	rng := rand.Reader
	message := []byte(text)
	hashed := sha256.Sum256(message)

	signature, err := rsa.SignPKCS1v15(rng, privateKey, crypto.SHA256, hashed[:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error from signing: %s\n", err)
		return "", err
	}

	return base64.StdEncoding.EncodeToString(signature), nil
}

// Key

func loadPrivateKey() (*rsa.PrivateKey, error) {
	priv, err := ioutil.ReadFile(authProfile.PrivateKeyPath)
	if err != nil {
		return nil, errors.New("no RSA private key found")
	}

	privPem, _ := pem.Decode(priv)
	if privPem.Type != "RSA PRIVATE KEY" {
		return nil, errors.New("RSA private key is of the wrong type, Pem Type:" + privPem.Type)
	}
	privPemBytes := privPem.Bytes

	var parsedKey interface{}
	if parsedKey, err = x509.ParsePKCS1PrivateKey(privPemBytes); err != nil {
		if parsedKey, err = x509.ParsePKCS8PrivateKey(privPemBytes); err != nil { // note this returns type `interface{}`
			return nil, errors.New("unable to parse RSA private key")
		}
	}

	var privateKey *rsa.PrivateKey
	var ok bool
	privateKey, ok = parsedKey.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("unable to parse RSA private key")
	}

	return privateKey, nil
}
