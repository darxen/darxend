package main

import (
	"io/ioutil"
	"fmt"
	"net/http"
	"os"
	"log"
	"github.com/darxen/goftp"
	"sort"
	"errors"
	"strings"
	"regexp"
	"strconv"
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
	http.HandleFunc("/", root)
	http.HandleFunc("/latest/", latest)
	http.HandleFunc("/ls/", ls)

	err := http.ListenAndServe(":" + port(), nil)
	if err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}

func root(w http.ResponseWriter, req *http.Request) {
	http.Redirect(w, req, "http://play.google.com/store/apps/details?id=me.kevinwells.darxen", 301)
}

func prune(entries []*ftp.Entry) (result []*ftp.Entry) {
	for _, entry := range entries {
		if entry.Name != "sn.last" {
			result = append(result, entry)
		}
	}
	return
}

func openConnection(site string) (conn *ftp.ServerConn, err error) {

	conn, err = ftp.Connect("tgftp.nws.noaa.gov:21")
	if err != nil {
		return nil, errors.New("Unable to connect")
	}
	defer func() {
		if err != nil && conn != nil {
			conn.Quit()
			conn = nil
		}
	}()

	err = conn.Login("anonymous", "darxen")
	if err != nil {
		return conn, errors.New("Unable to login")
	}

	path := fmt.Sprintf("SL.us008001/DF.of/DC.radar/DS.p19r0/SI.%s", site)
	err = conn.ChangeDir(path)
	if err != nil {
		return conn, errors.New("Unable to chdir")
	}

	return conn, nil
}

func closeConnection(conn *ftp.ServerConn) {
	conn.Quit()
}

func downloadFile(conn *ftp.ServerConn, path string) (data []byte, err error) {
	stream, err := conn.Retr("sn.last")
	if err != nil {
		return nil, errors.New("Unable to start transfer")
	}
	defer stream.Close()

	data, err = ioutil.ReadAll(stream)
	if err != nil {
		return nil, errors.New("Failed to read data file")
	}

	return data, nil
}

func loadEntries(conn *ftp.ServerConn) (entries []*ftp.Entry, err error) {
	entries, err = conn.List(".")
	if err != nil {
		return nil, errors.New("Unable to lookup directory")
	}

	sort.Sort(ftp.ByTime{entries})
	entries = prune(entries)

	return entries, nil
}

func handleClientError(w http.ResponseWriter, req *http.Request, msg string) {
	header := w.Header()
	header.Set("Content-Type", "text/html")
	w.WriteHeader(404)
	fmt.Fprintf(w, "<h1>404 Not Found</h1><h3>%s</h3>", msg)
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
	}
	site := parts[1]

	conn, err := openConnection(site)
	if err != nil {
		panic(err)
	}
	defer closeConnection(conn)

	var path string
	if len(parts) > 2 {
		//determine path based on the URL
		excluded := parts[2]

		valid, err := regexp.MatchString("sn\\.[0-9][0-9][0-9][0-9]", excluded)
		if err != nil || !valid {
		}

		re := regexp.MustCompile("^sn\\.(\\d\\d\\d\\d)$")
		v := re.FindStringSubmatch(excluded)
		if len(v) != 2 {
			handleClientError(w, req, "Invalid URL format")
			return
		}
		val, _ := strconv.Atoi(v[1])
		if val == 0 {
			val = 250
		} else {
			val -= 1
		}
		path = fmt.Sprintf("sn.%04d", val)

	} else {
		//determine path from a directory listing
		entries, err := loadEntries(conn)
		if err != nil {
			panic(err)
		}

		//use the latest entry
		entry := entries[len(entries)-1]
		path = entry.Name
	}

	data, err := downloadFile(conn, path)
	if err != nil {
		panic(err)
	}

	header := w.Header()
	header.Set("Content-Type", "application/octet-stream")
	header.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", path))
	header.Set("Filename", path)

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

