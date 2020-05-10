package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	lib_auth "github.com/taliesin-insa/lib-auth"
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
	"strconv"
	"testing"
)

var EmptyPiFF = PiFFStruct{
	Meta: Meta{
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

type VerifyRequest struct {
	Token string
}

var InsertedPath string
var InsertedRealFilename string

func MockAuthMicroservice() *httptest.Server {

	mockedServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/auth/verifyToken" {
				body, _ := ioutil.ReadAll(r.Body)

				var data VerifyRequest
				json.Unmarshal(body, &data)

				if data.Token == "chevreuil" {
					w.WriteHeader(http.StatusOK)
					r, _ := json.Marshal(lib_auth.UserData{Username: "morpheus", Role: lib_auth.RoleAdmin})
					w.Write(r)
					return
				} else {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
			}
		}))

	return mockedServer
}

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

func TestMain(m *testing.M) {
	mockedAuthServer := MockAuthMicroservice()

	previousAuthUrl := os.Getenv("AUTH_API_URL")
	os.Setenv("AUTH_API_URL", mockedAuthServer.URL)

	/* Mocking the shared folder */
	dir, createFolderErr := ioutil.TempDir("", "snippets")
	if createFolderErr != nil {
		panic("failed to create temp directory")
	}

	VolumePath = dir + "/"

	code := m.Run()

	removeErr := os.RemoveAll(VolumePath)
	if removeErr != nil {
		panic("failed to remove temp directory")
	}

	os.Setenv("AUTH_API_URL", previousAuthUrl)
	os.Exit(code)
}

func TestUploadImageBadAuth(t *testing.T) {
	wrongAuthRequest := &http.Request{
		Method: http.MethodPost,
		Header: map[string][]string{
			"Authorization": {"matrix"},
		},
	}

	// make http request
	wrongAuthRecorder := httptest.NewRecorder()
	createDatabase(wrongAuthRecorder, wrongAuthRequest)

	// status test
	assert.Equal(t, http.StatusBadRequest, wrongAuthRecorder.Code)
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

	/* Create some placeholder files in the shared folder */
	for i := 0; i < 5; i++ {
		file, createFileErr := ioutil.TempFile(VolumePath, strconv.Itoa(i))
		if createFileErr != nil {
			panic("could not create temp file")
		}

		file.Write([]byte(strconv.Itoa(i)))
	}

	/* Mocking Database API Response */
	mockedDBServer := MockDatabaseMicroservice()

	/* Mock http request, here in createDatabase we don't use the request struct so we can pass a blank one */
	request := &http.Request{
		Method: http.MethodPost,
		Header: map[string][]string{
			"Authorization": {"chevreuil"},
		},
	}

	/* Recorder object that saves the http feedback from the createDatabase function */
	recorder := httptest.NewRecorder()

	DatabaseAPI = mockedDBServer.URL
	createDatabase(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)

	files, _ := ioutil.ReadDir(VolumePath)

	assert.Len(t, files, 0, "createDatabase did not clear snippets folder")

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

func TestCreateDatabaseErrorErasingSnippets(t *testing.T) {
	request := &http.Request{
		Method: http.MethodPost,
		Header: map[string][]string{
			"Authorization": {"chevreuil"},
		},
	}
	recorder := httptest.NewRecorder()

	saveVolume := VolumePath
	VolumePath = "/invalid/folder"

	createDatabase(recorder, request)

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)

	VolumePath = saveVolume
}

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
		Header: map[string][]string{
			"Authorization": {"chevreuil"},
		},
	}
	recorder := httptest.NewRecorder()

	DatabaseAPI = ts.URL
	createDatabase(recorder, request)

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)

}

func TestUploadImageNoMultipartForm(t *testing.T) {
	request := &http.Request{
		Method: http.MethodPost,
		Header: map[string][]string{
			"Authorization": {"chevreuil"},
		},
		Body: nil,
	}
	recorder := httptest.NewRecorder()

	uploadImage(recorder, request)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)

	assert.Equal(t, "[MICRO-IMPORT] Couldn't parse multipart form (wrong format, network issues ?)", string(recorder.Body.Bytes()))

}

func generateMultipartForm(paramName string) (io.ReadCloser, string) {
	imageContent := new(bytes.Buffer)
	body := new(bytes.Buffer)

	pngEncodeErr := png.Encode(imageContent, createImage(200, 200))
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
		Body:   formBody,
		Header: map[string][]string{
			"Content-Type":  {formContentType},
			"Authorization": {"chevreuil"},
		},
	}

	recorder := httptest.NewRecorder()

	uploadImage(recorder, request)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestUploadImageTooLargeImageSize(t *testing.T) {
	body := new(bytes.Buffer)

	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "sample.png")
	part.Write(make([]byte, 32<<22))

	writer.Close()
	formBody := ioutil.NopCloser(body)
	formContentType := writer.FormDataContentType()

	request := &http.Request{
		Method: http.MethodPost,
		Body:   formBody,
		Header: map[string][]string{
			"Content-Type":  {formContentType},
			"Authorization": {"chevreuil"},
		},
	}

	recorder := httptest.NewRecorder()

	uploadImage(recorder, request)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestUploadImageInvalidImageExtension(t *testing.T) {
	formBody, formContentType := generateInvalidMultipartForm("file")
	request := &http.Request{
		Method: http.MethodPost,
		Body:   formBody,
		Header: map[string][]string{
			"Content-Type":  {formContentType},
			"Authorization": {"chevreuil"},
		},
	}

	recorder := httptest.NewRecorder()

	uploadImage(recorder, request)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestUploadImageMultipartForm(t *testing.T) {

	PodName = "podname"

	mockedConversionServer := MockConversionMicroservice()
	mockedDBServer := MockDatabaseMicroservice()

	ConversionAPI = mockedConversionServer.URL
	DatabaseAPI = mockedDBServer.URL

	formBody, formContentType := generateMultipartForm("file")
	request := &http.Request{
		Method: http.MethodPost,
		Body:   formBody,
		Header: map[string][]string{
			"Content-Type":  {formContentType},
			"Authorization": {"chevreuil"},
		},
	}

	recorder := httptest.NewRecorder()

	uploadImage(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)

	_, err := os.Stat(InsertedPath)
	assert.Nil(t, err, "file was not saved to volume")

	assert.Equal(t, "sample.png", InsertedRealFilename, "the name of the file before renaming was not correctly saved")

	os.RemoveAll(VolumePath)
	mockedConversionServer.Close()
	mockedDBServer.Close()
}
