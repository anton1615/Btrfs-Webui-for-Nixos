package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"io/ioutil"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

//go:embed ui/*
var embedUI embed.FS

type Snapshot struct {
	ID          int    `json:"id"`
	Type        string `json:"type"`
	PreID       string `json:"pre_id"`
	Date        string `json:"date"`
	User        string `json:"user"`
	Cleanup     string `json:"cleanup"`
	Description string `json:"description"`
	UserData    string `json:"userdata"`
}

type DiffEntry struct {
	Action string `json:"action"`
	Path   string `json:"path"`
}

func parseSnapshotList(input string) []Snapshot {
	lines := strings.Split(input, "\n")
	snapshots := []Snapshot{}
	// Updated regex to include 8th column: User Data
	re := regexp.MustCompile(`^\s*(\d+)\s*[|│]\s*(\w+)\s*[|│]\s*(\d*)\s*[|│]\s*([^|│]*)\s*[|│]\s*([^|│]*)\s*[|│]\s*([^|│]*)\s*[|│]\s*([^|│]*)\s*[|│]\s*([^|│]*)`)
	for i, line := range lines {
		if i < 2 || strings.TrimSpace(line) == "" {
			continue
		}
		match := re.FindStringSubmatch(line)
		if len(match) < 9 {
			continue
		}
		id, _ := strconv.Atoi(match[1])
		snapshots = append(snapshots, Snapshot{
			ID:          id,
			Type:        strings.TrimSpace(match[2]),
			PreID:       strings.TrimSpace(match[3]),
			Date:        strings.TrimSpace(match[4]),
			User:        strings.TrimSpace(match[5]),
			Cleanup:     strings.TrimSpace(match[6]),
			Description: strings.TrimSpace(match[7]),
			UserData:    strings.TrimSpace(match[8]),
		})
	}
	return snapshots
}

func parseDiffList(input string) []DiffEntry {
	lines := strings.Split(input, "\n")
	var entries []DiffEntry
	for _, line := range lines {
		if len(line) < 10 {
			continue
		}
		entries = append(entries, DiffEntry{Action: string(line[0]), Path: strings.TrimSpace(line[7:])})
	}
	return entries
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		data, _ := fs.ReadFile(embedUI, "ui/index.html")
		w.Header().Set("Content-Type", "text/html")
		w.Write(data)
	})

	http.HandleFunc("/api/configs", func(w http.ResponseWriter, r *http.Request) {
		files, _ := ioutil.ReadDir("/etc/snapper/configs")
		var configs []string
		for _, f := range files {
			configs = append(configs, f.Name())
		}
		json.NewEncoder(w).Encode(configs)
	})

	http.HandleFunc("/api/get-config", func(w http.ResponseWriter, r *http.Request) {
		config := r.URL.Query().Get("config")
		cmd := exec.Command("snapper", "-c", config, "get-config")
		output, _ := cmd.Output()
		lines := strings.Split(string(output), "\n")
		res := make(map[string]string)
		for _, line := range lines {
			parts := strings.Split(line, "│")
			if len(parts) == 2 {
				res[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
		json.NewEncoder(w).Encode(res)
	})

	http.HandleFunc("/api/snapshots", func(w http.ResponseWriter, r *http.Request) {
		config := r.URL.Query().Get("config")
		cmd := exec.Command("snapper", "-c", config, "list")
		output, _ := cmd.Output()
		json.NewEncoder(w).Encode(parseSnapshotList(string(output)))
	})

	http.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		config := r.URL.Query().Get("config")
		rangeStr := r.URL.Query().Get("range")
		cmd := exec.Command("snapper", "-c", config, "status", rangeStr)
		output, _ := cmd.Output()
		json.NewEncoder(w).Encode(parseDiffList(string(output)))
	})

	http.HandleFunc("/api/undochange", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			return
		}
		var req struct {
			Config string   `json:"config"`
			Range  string   `json:"range"`
			Paths  []string `json:"paths"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		args := []string{"-c", req.Config, "undochange", req.Range, "--"}
		args = append(args, req.Paths...)
		exec.Command("snapper", args...).Run()
		w.WriteHeader(200)
	})

	http.HandleFunc("/api/create", func(w http.ResponseWriter, r *http.Request) {
		config := r.URL.Query().Get("config")
		desc := r.URL.Query().Get("description")
		userdata := r.URL.Query().Get("userdata")
		args := []string{"-c", config, "create", "--description", desc}
		if userdata != "" {
			args = append(args, "--userdata", userdata)
		}
		err := exec.Command("snapper", args...).Run()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.WriteHeader(201)
	})

	http.HandleFunc("/api/delete", func(w http.ResponseWriter, r *http.Request) {
		config := r.URL.Query().Get("config")
		id := r.URL.Query().Get("id")
		exec.Command("snapper", "-c", config, "delete", id).Run()
		w.WriteHeader(200)
	})

	fmt.Println("Btrfs Console Server Running on :8888")
	http.ListenAndServe(":8888", nil)
}