package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"regexp"
	"sort"
	"strconv"

	"github.com/darxen/goftp"
)

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

func extractIndex(path string) string {
	re := regexp.MustCompile("^sn\\.(\\d\\d\\d\\d)$")
	v := re.FindStringSubmatch(path)
	if len(v) != 2 {
		return ""
	}
	return v[1]
}

func previousPath(path string) string {
	v := extractIndex(path)
	if v == "" {
		return ""
	}
	val, _ := strconv.Atoi(v)
	if val == 0 {
		val = 250
	} else {
		val -= 1
	}
	return fmt.Sprintf("sn.%04d", val)
}
