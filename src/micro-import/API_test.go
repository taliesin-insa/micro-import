package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
)

var EmptyPiFF = PiFFStruct{
	Meta:     Meta{
		Type: "line",
		URL:  "",
	},
	Location: []Location{
		{Type: "line",
			Polygon: [][2]int{
				{0, 0},
				{0, 0},
				{0, 0},
				{0, 0},
			},
			Id: "loc_0",
		},
	},
	Data: []Data{
		{
			Type:       "line",
			LocationId: "loc_0",
			Value:      "",
			Id:         "0",
		},
	},
	Children: nil,
	Parent:   0,
}

var InsertedPath string
var InsertedRealFilename string

func MockDatabaseMicroservice() *httptest.Server {

	mockedServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/db/delete/all" {
				w.WriteHeader(http.StatusOK)
			} else if r.URL.Path == "/db/insert" {
				body, _ := ioutil.ReadAll(r.Body)

				var data = []DBEntry{}
				json.Unmarshal(body, &data)

				if !reflect.DeepEqual(data[0].PiFF, EmptyPiFF) {
					w.WriteHeader(http.StatusBadRequest)
				} else {
					w.WriteHeader(http.StatusCreated)
				}

				InsertedPath = data[0].Url
				InsertedRealFilename = data[0].Filename

			}
		}))

	return mockedServer
}

func MockConversionMicroservice() *httptest.Server {

	mockedServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/convert/nothing" {
				w.WriteHeader(http.StatusOK)
				body, _ := json.Marshal(EmptyPiFF)
				w.Write(body)
			}
		}))

	return mockedServer
}


func TestCreateDatabaseOK(t *testing.T) {

	/* Mocking Database API Response */
	mockedDBServer := MockDatabaseMicroservice()

	/* Mock http request, here in createDatabase we don't use the request struct so we can pass a blank one */
	request := &http.Request{
		Method: http.MethodPost,
	}

	/* Recorder object that saves the http feedback from the createDatabase function */
	recorder := httptest.NewRecorder()

	DatabaseAPI = mockedDBServer.URL
	createDatabase(recorder, request)

	if status := recorder.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
		return
	}

	mockedDBServer.Close()
}

func createImage(width int, height int) *image.RGBA {
	upLeft := image.Point{0, 0}
	lowRight := image.Point{X: width, Y: height}

	img := image.NewRGBA(image.Rectangle{Min: upLeft, Max: lowRight})

	// set color for each pixel
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, color.White)
		}
	}

	return img
}

// FIXME : at the moment the calls made in createDatabase won't ever return an error
func TestCreateDatabaseErrorFromDBAPI(t *testing.T) {

	ts := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/db/delete/all" {
				/* Mocking a failure from the Database */
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "[MICRO-DATABASE] MongoDB timed out")
			}
		}))

	defer ts.Close()

	request := &http.Request{
		Method: http.MethodPost,
	}
	recorder := httptest.NewRecorder()

	DatabaseAPI = ts.URL
	createDatabase(recorder, request)

	if status := recorder.Code; status != http.StatusInternalServerError {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusInternalServerError)
		return
	}

}

func TestUploadImageNoMultipartForm(t *testing.T) {
	request := &http.Request{
		Method: http.MethodPost,
		Body: nil,
	}
	recorder := httptest.NewRecorder()

	uploadImage(recorder, request)

	if status := recorder.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
		return
	}

	if message := string(recorder.Body.Bytes()) ; message != "[MICRO-IMPORT] Couldn't parse multipart form (wrong format, network issues ?)" {
		t.Errorf("handler returned wrong response body: got %v want %v",
			message, "[MICRO-IMPORT] Couldn't parse multipart form (wrong format, network issues ?)")
		return
	}

}

func generateMultipartForm(paramName string) (io.ReadCloser, string) {
	imageContent := new(bytes.Buffer)
	body := new(bytes.Buffer)

	pngEncodeErr := png.Encode(imageContent, createImage(200,200))
	if pngEncodeErr != nil {
		panic("could not generate multipart form")
	}

	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile(paramName, "sample.png")
	part.Write(imageContent.Bytes())
	writer.Close()
	return ioutil.NopCloser(body), writer.FormDataContentType()
}

func generateInvalidMultipartForm(paramName string) (io.ReadCloser, string) {
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile(paramName, "sample.txt")
	part.Write([]byte("test"))
	writer.Close()
	return ioutil.NopCloser(body), writer.FormDataContentType()
}


func TestUploadImageInvalidMultipartForm(t *testing.T) {
	formBody, formContentType := generateMultipartForm("notfile")
	request := &http.Request{
		Method: http.MethodPost,
		Body: formBody,
		Header: map[string][]string{
			"Content-Type": {formContentType},
		},
	}

	recorder := httptest.NewRecorder()

	uploadImage(recorder, request)

	if status := recorder.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
		return
	}
}

func TestUploadImageInvalidImageExtension(t *testing.T) {
	formBody, formContentType := generateInvalidMultipartForm("file")
	request := &http.Request{
		Method: http.MethodPost,
		Body: formBody,
		Header: map[string][]string{
			"Content-Type": {formContentType},
		},
	}

	recorder := httptest.NewRecorder()

	uploadImage(recorder, request)

	if status := recorder.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
		return
	}
}

func TestUploadImageMultipartForm(t *testing.T) {
	VolumePath, _ = ioutil.TempDir("", "")
	VolumePath+="/"

	PodName = "podname"

	mockedConversionServer := MockConversionMicroservice()
	mockedDBServer := MockDatabaseMicroservice()

	ConversionAPI = mockedConversionServer.URL
	DatabaseAPI = mockedDBServer.URL

	formBody, formContentType := generateMultipartForm("file")
	request := &http.Request{
		Method: http.MethodPost,
		Body: formBody,
		Header: map[string][]string{
			"Content-Type": {formContentType},
		},
	}

	recorder := httptest.NewRecorder()

	uploadImage(recorder, request)

	if status := recorder.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
		return
	}

	if _, err := os.Stat(InsertedPath); err != nil {
		t.Error("file was not saved to volume")
		return
	}

	if InsertedRealFilename != "sample.png" {
		t.Errorf("the name of the file before renaming was not correctly saved, got %v want %v", InsertedRealFilename, "sample.txt")
		return
	}

	os.RemoveAll(VolumePath)
	mockedConversionServer.Close()
	mockedDBServer.Close()
}
