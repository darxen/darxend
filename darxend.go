package main

import (
	"io/ioutil"
	"fmt"
	"net/http"
	"os"
	"log"
	"github.com/jlaffaye/goftp"
)

var DEBUG bool = true

func port() (res string) {
	defer func() {
		if DEBUG {
			fmt.Printf("Running on port: %s\n", res)
		}
	}()
	var port = os.Getenv("PORT")
	if port != "" {
		return port
	}
	return "5000"
}

func main() {
	http.HandleFunc("/", hello)
	http.HandleFunc("/test", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "/test/", 301)
	})
	http.HandleFunc("/test/", test)

	err := http.ListenAndServe(":" + port(), nil)
	if err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}

func hello(w http.ResponseWriter, req *http.Request) {
	var header = w.Header()
	header.Set("Content-Type", "text/html")

	fmt.Fprintf(w, "<h2>Hello <a href='/test'>world</a>!</h2>")
}

func test(w http.ResponseWriter, req *http.Request) {

	defer func() {
		if r := recover(); r != nil {
			w.WriteHeader(501)
			fmt.Fprintf(w, "Error: %s", r)
		}
	}()

	conn, err := ftp.Connect("tgftp.nws.noaa.gov:21")
	if err != nil {
		panic("Unable to connect")
	}

	err = conn.Login("anonymous", "darxen")
	if err != nil {
		panic("Unable to login")
	}

	err = conn.ChangeDir("SL.us008001/DF.of/DC.radar/DS.p19r0/SI.klot")
	if err != nil {
		panic("Unable to chdir")
	}

	stream, err := conn.Retr("sn.last")
	if err != nil {
		panic("Unable to start transfer")
	}

	data, err := ioutil.ReadAll(stream)
	if err != nil {
		panic("Failed to read data file")
	}

	err = stream.Close()
	conn.Quit()

	//write the response
	header := w.Header()
	header.Set("Content-Type", "application/octet-stream")
	_, err = w.Write(data)

}

