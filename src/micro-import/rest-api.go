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
	"path/filepath"
	"time"
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
	Url      string     `json:"Url"`      //The URL on our fileserver
	Filename string     `json:"Filename"` //The original name of the file
	// Flags
	Annotated  bool `json:"Annotated"`
	Corrected  bool `json:"Corrected"`
	SentToReco bool `json:"SentToReco"`
	SentToUser bool `json:"SentToUser"`
	Unreadable bool `json:"Unreadable"`
	Annotator string `json:"Annotator"`
}


func home(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "you're talking to the import microservice")
}

func createDatabase(w http.ResponseWriter, r *http.Request) {

	// FIXME : for v0 we erase previous data in db, needs to be changed later
	client := &http.Client{}
	eraseRequest, _ := http.NewRequest(http.MethodDelete, DatabaseAPI+"/db/delete/all", nil)
	eraseResponse, eraseErr := client.Do(eraseRequest)

	if eraseErr == nil && eraseResponse.StatusCode == http.StatusOK {
		w.WriteHeader(http.StatusOK)
	} else if eraseErr != nil {
		log.Printf("[ERROR] Error in call to db/delete/all: %v", eraseErr.Error())
		fmt.Fprint(w ,"[MICRO-IMPORT] Error in request to database")
	} else {
		w.WriteHeader(http.StatusInternalServerError)
		// error returned by db api
		eraseResponseBody, _ := ioutil.ReadAll(eraseResponse.Body)
		log.Printf("[ERROR] Error in response from db/delete/all: %v", string(eraseResponseBody))
		fmt.Fprint(w ,"[MICRO-IMPORT] Error in response from database")
	}

}

func uploadImage(w http.ResponseWriter, r *http.Request) {
	parseError := r.ParseMultipartForm(MaxImageSize)

	if parseError != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Printf("[ERROR] Parse multipart form: %v", parseError.Error())
		fmt.Fprint(w ,"[MICRO-IMPORT] Couldn't parse multipart form (wrong format, network issues ?)")
		return
	}

	formFile, formFileHeader, formFileErr := r.FormFile("file")

	if formFileErr == nil {
		defer formFile.Close()

		now := time.Now()
		nsec := now.UnixNano()
		podname, hostnameErr := os.Hostname()

		if hostnameErr != nil {
			panic("[PANIC] Could not get hostname")
		}

		extension := filepath.Ext(formFileHeader.Filename)
		path := VolumePath+string(nsec)+podname+"."+extension
		fmt.Println(path)

		file, fileErr := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0666)

		if fileErr == nil {
			io.Copy(file, formFile)
			file.Close() // closing file now so it can be read by conversion

			piffReq := PiFFRequest{Path:path}
			piffReqJson, _ := json.Marshal(piffReq)

			convertRes, convertErr := http.Post(ConversionAPI+"/convert/nothing", "application/json", bytes.NewBuffer(piffReqJson))

			if convertErr != nil {
				w.WriteHeader(http.StatusInternalServerError)
				log.Printf("[ERROR] Error in call to convert/nothing: %v", convertErr.Error())
				fmt.Fprint(w ,"[MICRO-IMPORT] Error in request to conversion")
				return
			}

			convertResBody, convertResErr := ioutil.ReadAll(convertRes.Body)

			if convertResErr != nil {
				w.WriteHeader(http.StatusInternalServerError)
				log.Printf("[ERROR] Error parsing convert/nothing: %v", convertResErr.Error())
				fmt.Fprint(w ,"[MICRO-IMPORT] Error parsing response from conversion")
				return
			}

			var piff PiFFStruct
			unmarshallErr := json.Unmarshal(convertResBody, &piff)

			dbEntry := DBEntry{
				PiFF: piff,
				Url:  path,
				Filename: formFileHeader.Filename,
			}

			if unmarshallErr != nil {
				w.WriteHeader(http.StatusInternalServerError)
				log.Printf("[ERROR] Error unmarshalling json received from conversion : %v", unmarshallErr.Error())
				fmt.Fprint(w ,"[MICRO-IMPORT] Error parsing response from conversion")
				return
			}

			dbListOfEntries := [1]DBEntry{dbEntry}
			mDbEntry, _ := json.Marshal(dbListOfEntries)

			dbInsertRes, dbInsertErr := http.Post(DatabaseAPI+"/db/insert", "application/json", bytes.NewBuffer(mDbEntry))

			if dbInsertErr == nil && dbInsertRes.StatusCode == http.StatusCreated {
				_, dbReadErr := ioutil.ReadAll(dbInsertRes.Body)

				if dbReadErr != nil {
					w.WriteHeader(http.StatusInternalServerError)
					log.Printf("[ERROR] Error parsing db/insert: %v", dbReadErr.Error())
					fmt.Fprint(w ,"[MICRO-IMPORT] Error parsing response from database")
					return
				}

				w.WriteHeader(http.StatusOK)
			} else {
				// error returned by db api
				w.WriteHeader(http.StatusInternalServerError)
				log.Printf("[ERROR] Error in call to db/insert: %v", dbInsertErr.Error())
				fmt.Fprint(w ,"[MICRO-IMPORT] Error in request to database")
				return
			}

		} else {
			// creating file on volume error
			w.WriteHeader(http.StatusInternalServerError)
			log.Printf("[ERROR] Cannot open file %v: %v", path, fileErr.Error())
			fmt.Fprintf(w ,"[MICRO-IMPORT] Couldn't write provided file %v)", path)
			return
		}

	} else {
		// file upload/multipart form error
		w.WriteHeader(http.StatusBadRequest)
		log.Printf("[ERROR] Parse multipart form: %v", formFileErr.Error())
		fmt.Fprint(w ,"[MICRO-IMPORT] Couldn't parse multipart form (key file probably missing/unreadable)")
		return
	}
}

func main() {
	dbEnvVal, dbEnvExists := os.LookupEnv("DATABASE_API_URL")
	convertEnvVal, convertEnvExists := os.LookupEnv("CONVERSION_API_URL")

	if dbEnvExists {
		DatabaseAPI = dbEnvVal
	} else {
		DatabaseAPI = "http://database-api.gitlab-managed-apps.svc.cluster.local:8080"
	}

	if convertEnvExists {
		ConversionAPI = convertEnvVal
	} else {
		ConversionAPI = "http://conversion-api.gitlab-managed-apps.svc.cluster.local:12345"
	}

	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/import/", home)

	router.HandleFunc("/import/createDB", createDatabase).Methods("POST")
	router.HandleFunc("/import/upload", uploadImage).Methods("POST")

	log.Fatal(http.ListenAndServe(":8080", router))
}
