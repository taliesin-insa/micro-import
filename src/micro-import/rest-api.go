package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

const MaxImageSize = 32 << 20
var VolumePath = "/snippets/"

var DatabaseAPI string
var ConversionAPI string

type Meta struct {
	Type string
	URL  string
}

type Location struct {
	Type    string
	Polygon [][2]int
	Id      string
}

type Data struct {
	Type       string
	LocationId string
	Value      string
	Id         string
}

type PiFFStruct struct {
	Meta     Meta
	Location []Location
	Data     []Data
	Children []int
	Parent   int
}

type PiFFRequest struct {
	Path string
}

type DBEntry struct {
	// Piff
	PiFF PiFFStruct `json:"PiFF"`
	// Url fileserver
	Url string `json:"Url"`
	// Flags
	Annotated  bool `json:"Annotated"`
	Corrected  bool `json:"Corrected"`
	SentToReco bool `json:"SentToReco"`
	SentToUser bool `json:"SentToUser"`
	Unreadable bool `json:"Unreadable"`
}


func home(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "you're talking to the import microservice")
}

func createDatabase(w http.ResponseWriter, r *http.Request) {

	// FIXME : for v0 we erase previous data in db, needs to be changed later
	client := &http.Client{}
	eraseRequest, _ := http.NewRequest(http.MethodPut, DatabaseAPI+"/db/delete/all", nil)
	eraseResponse, eraseErr := client.Do(eraseRequest)

	if eraseErr == nil && eraseResponse.StatusCode == http.StatusAccepted {
		w.WriteHeader(http.StatusOK)
	} else if eraseErr != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, eraseErr.Error())
	} else {
		w.WriteHeader(http.StatusInternalServerError)
		// error returned by db api
		fmt.Fprint(w, eraseResponse.Body)
	}

}

func uploadImage(w http.ResponseWriter, r *http.Request) {
	parseError := r.ParseMultipartForm(MaxImageSize)

	if parseError != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Println(parseError)
		fmt.Fprint(w, parseError.Error())
		return
	}

	formFile, formFileHeader, formFileErr := r.FormFile("file")

	if formFileErr == nil {
		defer formFile.Close()

		// FIXME : we use the filename provided by the user, input check or decide of a naming policy for files
		path := VolumePath+formFileHeader.Filename

		fmt.Println(path)
		file, fileErr := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0666)

		if fileErr == nil {
			defer file.Close()
			io.Copy(file, formFile)

			piffReq := PiFFRequest{Path:path}
			piffReqJson, _ := json.Marshal(piffReq)

			convertRes, convertErr := http.Post(ConversionAPI+"/convert/nothing", "application/json", bytes.NewBuffer(piffReqJson))

			if convertErr != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Println(convertErr)
				fmt.Fprint(w, convertErr.Error())
				return
			}

			convertResBody, convertResErr := ioutil.ReadAll(convertRes.Body)

			if convertResErr != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Println(convertResErr)
				fmt.Fprint(w, convertResErr.Error())
				return
			}

			var piff PiFFStruct
			unmarshallErr := json.Unmarshal(convertResBody, &piff)

			dbEntry := DBEntry{
				PiFF: piff,
				Url:  path,
			}

			if unmarshallErr != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Println(unmarshallErr)
				fmt.Fprint(w, unmarshallErr.Error())
			}

			// TODO: ask for a single Picture endpoint in db microservice

			dbListOfEntries := [1]DBEntry{dbEntry}
			mDbEntry, _ := json.Marshal(dbListOfEntries)

			dbInsertRes, dbInsertErr := http.Post(DatabaseAPI+"/db/insert", "application/json", bytes.NewBuffer(mDbEntry))

			if dbInsertErr == nil && dbInsertRes.StatusCode == http.StatusCreated {
				_, dbReadErr := ioutil.ReadAll(dbInsertRes.Body)

				if dbReadErr != nil {
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Println(dbReadErr)
					fmt.Fprint(w, dbReadErr.Error())
					return
				}

				w.WriteHeader(http.StatusOK)
			} else {
				// error returned by db api
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Println(dbInsertErr)
				fmt.Fprint(w, dbInsertErr.Error())
			}

		} else {
			// creating file on volume error
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Println(fileErr)
			fmt.Fprint(w, fileErr.Error())
		}

	} else {
		// file upload/multipart form error
		w.WriteHeader(http.StatusBadRequest)
		fmt.Println(formFileErr)
		fmt.Fprint(w, formFileErr.Error())
	}
}

func main() {

	if os.Getenv("MICRO_ENVIRONMENT") == "production" {
		DatabaseAPI  = "http://database-api.gitlab-managed-apps.svc.cluster.local:8080"
		ConversionAPI = "http://conversion-api.gitlab-managed-apps.svc.cluster.local:12345"
		fmt.Println("Started in production environment.")
	} else {
		DatabaseAPI = "http://database-api-dev.gitlab-managed-apps.svc.cluster.local:8080"
		ConversionAPI = "http://conversion-api-dev.gitlab-managed-apps.svc.cluster.local:12345"
		fmt.Println("Started in dev environment.")
	}

	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/", home)

	router.HandleFunc("/createDB", createDatabase).Methods("POST")
	router.HandleFunc("/upload", uploadImage).Methods("POST")

	log.Fatal(http.ListenAndServe(":8080", router))
}
