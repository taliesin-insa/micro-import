package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateDatabaseOK(t *testing.T) {

	/* Mocking Database API Response */
	ts := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/db/delete/all" {
				w.WriteHeader(http.StatusAccepted)
			}
		}))

	defer ts.Close()

	/* Mock http request, here in createDatabase we don't use the request struct so we can pass a blank one */
	request := &http.Request{}

	/* Recorder object that saves the http feedback from the createDatabase function */
	recorder := httptest.NewRecorder()

	DatabaseAPI = ts.URL
	createDatabase(recorder, request)

	if status := recorder.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
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

	request := &http.Request{}
	recorder := httptest.NewRecorder()

	DatabaseAPI = ts.URL
	createDatabase(recorder, request)

	if status := recorder.Code; status != http.StatusInternalServerError {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

}
