package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
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

func serve() {
	http.HandleFunc("/", root)
	http.HandleFunc("/latest/", latest)
	http.HandleFunc("/before/", before)
	http.HandleFunc("/ls/", ls)

	err := http.ListenAndServe(":"+port(), nil)
	if err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}

func root(w http.ResponseWriter, req *http.Request) {
	http.Redirect(w, req, "http://play.google.com/store/apps/details?id=me.kevinwells.darxen", 301)
}

func latest(w http.ResponseWriter, req *http.Request) {
	defer func() {
		if r := recover(); r != nil {
			err, _ := r.(error)
			header := w.Header()
			header.Set("Content-Type", "text/html")
			w.WriteHeader(501)
			fmt.Fprintf(w, "<h1>501 Internal Server Error</h1><h3>%s</h3>", err)
		}
	}()

	parts := strings.Split(strings.Trim(req.URL.Path, "/"), "/")
	if len(parts) < 2 {
		handleClientError(w, req, "No radar site provided")
		return
	} else if len(parts) < 3 {
		handleClientError(w, req, "No product provided")
		return
	}
	site := parts[1]
	product := parts[2]

	if product != "N0R" {
		handleClientError(w, req, "Invalid product type")
		return
	}

	//determine the latest file stored by the client
	var current string
	if len(parts) > 3 {
		current = parts[3]
		if extractIndex(current) == "" {
			handleClientError(w, req, "Invalid URL format")
			return
		}
	}

	conn, err := openConnection(site)
	if err != nil {
		panic(err)
	}
	defer closeConnection(conn)

	//determine path from a directory listing
	entries, err := loadEntries(conn)
	if err != nil {
		panic(err)
	}

	//use the latest entry
	entry := entries[len(entries)-1]
	path := entry.Name
	prev := previousPath(path)

	//check if we need to download the file
	if current != "" {
		if current == path {
			handleNoContent(w, req)
			return
		}
	}

	data, err := downloadFile(conn, path)
	if err != nil {
		panic(err)
	}

	header := w.Header()
	header.Set("Content-Type", "application/octet-stream")
	header.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", path))
	header.Set("Filename", path)
	header.Set("Previous-Filename", prev)

	w.Write(data)
}

func before(w http.ResponseWriter, req *http.Request) {
	defer func() {
		if r := recover(); r != nil {
			err, _ := r.(error)
			header := w.Header()
			header.Set("Content-Type", "text/html")
			w.WriteHeader(501)
			fmt.Fprintf(w, "<h1>501 Internal Server Error</h1><h3>%s</h3>", err)
		}
	}()

	parts := strings.Split(strings.Trim(req.URL.Path, "/"), "/")
	if len(parts) < 2 {
		handleClientError(w, req, "No radar site provided")
		return
	} else if len(parts) < 3 {
		handleClientError(w, req, "No product provided")
		return
	} else if len(parts) < 4 {
		handleClientError(w, req, "No filename provided")
		return
	}
	site := parts[1]
	product := parts[2]
	excluded := parts[3]

	if product != "N0R" {
		handleClientError(w, req, "Invalid product type")
		return
	}

	path := previousPath(excluded)
	if path == "" {
		handleClientError(w, req, "Invalid URL format")
		return
	}

	prev := previousPath(path)

	conn, err := openConnection(site)
	if err != nil {
		panic(err)
	}
	defer closeConnection(conn)

	data, err := downloadFile(conn, path)
	if err != nil {
		panic(err)
	}

	header := w.Header()
	header.Set("Content-Type", "application/octet-stream")
	header.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", path))
	header.Set("Filename", path)
	header.Set("Previous-Filename", prev)

	w.Write(data)
}

func ls(w http.ResponseWriter, req *http.Request) {

	defer func() {
		if r := recover(); r != nil {
			err, _ := r.(error)
			header := w.Header()
			header.Set("Content-Type", "text/html")
			w.WriteHeader(501)
			fmt.Fprintf(w, "<h1>501 Internal Server Error</h1><h3>%s</h3>", err)
		}
	}()

	conn, err := openConnection("klot")
	if err != nil {
		panic(err)
	}
	defer closeConnection(conn)

	entries, err := loadEntries(conn)
	if err != nil {
		panic(err)
	}

	header := w.Header()
	header.Set("Content-Type", "text/html")
	fmt.Fprintf(w, "<ul>")
	for _, entry := range entries {
		fmt.Fprintf(w, "<li>%s - %d - %s</li>", entry.Name, entry.Size, entry.Time)
	}
	fmt.Fprintf(w, "</ul>")
}

func handleNoContent(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(304)
}

func handleClientError(w http.ResponseWriter, req *http.Request, msg string) {
	header := w.Header()
	header.Set("Content-Type", "text/html")
	w.WriteHeader(404)
	fmt.Fprintf(w, "<h1>404 Not Found</h1><h3>%s</h3>", msg)
}
