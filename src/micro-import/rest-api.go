package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"io"
	"log"
	"net/http"
	"os"
)

const MaxImageSize = 32 << 20
const VolumePath = "/snippets/"

type Picture struct {
	Value string
	Url string
}

/*type UploadResponse struct {
	Path string
	// TODO: include error info
}*/

func home(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "you're talking to the import microservice")
}

func createDatabase(w http.ResponseWriter, r *http.Request) {

	/*// FIXME : for v0 we erase previous data in db, needs to be changed later
	eraseResponse, eraseErr := http.Get("http://database:8080/db/delete/all")

	if eraseErr == nil && eraseResponse.StatusCode == http.StatusAccepted {
		w.WriteHeader(http.StatusCreated)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
		// error returned by db api
	}
	// TODO : write a json response with potential errors*/
	w.WriteHeader(http.StatusOK)
}

func uploadImage(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(MaxImageSize)

	formFile, formFileHeader, formFileErr := r.FormFile("file")

	if formFileErr == nil {
		defer formFile.Close()

		// FIXME : we use the filename provided by the user, input check or decide of a naming policy for files
		path := VolumePath+formFileHeader.Filename

		file, fileErr := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0666)

		if fileErr == nil {
			defer file.Close()
			io.Copy(file, formFile)

			dbEntry := Picture{
				Value: "",
				Url:   path,
			}

			mDbEntry, _ := json.Marshal(dbEntry)

			/*
			// TODO: ask for a single Picture endpoint in db microservice
			dbInsertRes, dbInsertErr := http.Post("http://database:8080/db/insert", "application/json", bytes.NewBuffer(mDbEntry))

			if dbInsertErr == nil && dbInsertRes.StatusCode == http.StatusCreated {
				uploadRes := UploadResponse{
					Path: path,
				}

				mUploadRes, _ := json.Marshal(uploadRes)

				w.WriteHeader(http.StatusCreated)
				fmt.Fprint(w, mUploadRes)
			} else {
				// error returned by db api
				w.WriteHeader(http.StatusInternalServerError)
			}*/

			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, mDbEntry)

		} else {
			// creating file on volume error
			w.WriteHeader(http.StatusInternalServerError)
		}

	} else {
		// file upload/multipart form error
		w.WriteHeader(http.StatusNotAcceptable)
	}
}

func main() {

	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/", home)

	router.HandleFunc("/createDB", createDatabase).Methods("POST")
	router.HandleFunc("/upload", uploadImage).Methods("POST")

	log.Fatal(http.ListenAndServe(":8080", router))
}

