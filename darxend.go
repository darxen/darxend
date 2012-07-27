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

	conn, err := openConnection("klot")
	if err != nil {
		panic(err)
	}
	defer closeConnection(conn)

	entries, err := loadEntries(conn)
	if err != nil {
		panic(err)
	}

	url := req.URL
	parts := strings.Split(strings.Trim(url.Path, "/"), "/")
	var entry *ftp.Entry
	if len(parts) > 1 {
		//find the entry before the one specified
		excluded := parts[1]
		entry = func() *ftp.Entry {
			for i := len(entries)-1; i > 0; i -= 1 {
				if excluded == entries[i].Name {
					return entries[i-1]
				}
			}
			return nil
		}()
	} else {
		//use the latest entry
		entry = entries[len(entries)-1]
	}

	if entry == nil {
		header := w.Header()
		header.Set("Content-Type", "text/html")
		w.WriteHeader(401)
		fmt.Fprintf(w, "<h1>404 Not Found</h1>")
		return
	}

	data, err := downloadFile(conn, entry.Name)
	if err != nil {
		panic(err)
	}

	header := w.Header()
	header.Set("Content-Type", "application/octet-stream")
	header.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", entry.Name))
	header.Set("Filename", entry.Name)

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

