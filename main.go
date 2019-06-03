package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
)

var data = map[string]map[string]map[string]string{}

const maxUploadSize = 20 * 1024 * 1024 // 20 MB
const path = "./shapes.json"
const uploadPath = "./cache"

// load json into data
func init() {
	jsonFile, err := os.Open(path)
	if err != nil {
		logrus.Fatal(err)
	}
	defer jsonFile.Close()
	byteValue, _ := ioutil.ReadAll(jsonFile)
	json.Unmarshal(byteValue, &data)
}

// type Result map[string]map[string]map[string]string

func check(e error) {
	if e != nil {
		panic(e)
	}
}

// show all current shapes
func all(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(data)
	return
}

// get a shape or add a shape
func fileServe(w http.ResponseWriter, r *http.Request) {
	// params should be in the format /:region/:version/:shape
	params := strings.Split(r.URL.Path, "/")
	fmt.Println(params)
	if len(params) != 4 {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}
	region, version, shape := params[1], params[2], params[3]
	// only allowed 2 methods GET and POST
	switch r.Method {
	case "GET":
		// if the shape exists - serve
		if loc, ok := data[region][version][shape]; ok {
			http.ServeFile(w, r, loc)
			return
		}
		http.Error(w, "404 not found.", http.StatusNotFound)
		return

	case "POST":
		newpath := filepath.Join(uploadPath, region, version)

		if _, err := os.Stat(newpath); os.IsNotExist(err) {
			os.MkdirAll(newpath, os.ModePerm)
		}
		if _, ok := data[region]; !ok {
			data[region] = make(map[string]map[string]string)
		}
		if _, ok := data[region][version]; !ok {
			data[region][version] = make(map[string]string)
		}

		// if the file already exists return 400
		if path, ok := data[region][version][shape]; ok {
			fmt.Println(path)
			http.Error(w, "FILE_ALREADY_EXISTS", http.StatusBadRequest)
			return
		}

		// check file size - limit 20mb
		r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
		if err := r.ParseMultipartForm(maxUploadSize); err != nil {
			fmt.Println(err)
			http.Error(w, "FILE_TOO_BIG", http.StatusBadRequest)

			return
		}

		file, header, err := r.FormFile("uploadFile")
		filename := header.Filename
		if err != nil {
			http.Error(w, "INVALID_FILE", http.StatusBadRequest)
			return
		}
		defer file.Close()
		fileBytes, err := ioutil.ReadAll(file)
		if err != nil {
			http.Error(w, "INVALID_FILE", http.StatusBadRequest)
			return
		}

		// put file in cache
		newPath := filepath.Join(newpath, filename)
		newFile, err := os.Create(newPath)
		if err != nil {
			fmt.Println(err)

			http.Error(w, "CANT_WRITE_FILE", http.StatusInternalServerError)
			return
		}
		defer newFile.Close()

		if _, err := newFile.Write(fileBytes); err != nil {
			http.Error(w, "CANT_WRITE_FILE", http.StatusInternalServerError)
			return
		}

		// assign path to object
		data[region][version][shape] = newPath
		w.Write([]byte("SUCCESS"))

		// serialize the new data file
		bytes, err := json.Marshal(data)
		check(err)

		f, err := os.Create(path)
		defer f.Close()
		check(err)
		n, err := f.Write(bytes)
		fmt.Printf("wrote %d bytes", n)
		check(err)
		f.Sync()

	default:
		fmt.Fprintf(w, "Sorry, only GET and POST methods are supported.")
	}
}

func main() {

	http.HandleFunc("/all", all)
	http.HandleFunc("/", fileServe)

	fmt.Printf("Starting server for testing HTTP POST...\n")
	if err := http.ListenAndServe(":9004", nil); err != nil {
		log.Fatal(err)
	}
}
