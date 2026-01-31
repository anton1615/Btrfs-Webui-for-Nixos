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
	
	// Snapper list output table usually has headers on the first 2 lines
	// The separator is usually " | " or " │ "
	
	for i, line := range lines {
		if i < 2 || strings.TrimSpace(line) == "" {
			continue
		}
		
		// Normalize separators to pipe
		line = strings.ReplaceAll(line, "│", "|")
		parts := strings.Split(line, "|")
		
		// Expecting at least 7 columns (ID, Type, Pre, Date, User, Cleanup, Desc)
		// Userdata is 8th column (optional in output format depending on ver)
		if len(parts) < 7 {
			continue
		}

		idStr := strings.TrimSpace(parts[0])
		id, err := strconv.Atoi(idStr)
		if err != nil {
			continue // Skip if ID is not a number (e.g. current row marker)
		}

		snap := Snapshot{
			ID:          id,
			Type:        strings.TrimSpace(parts[1]),
			PreID:       strings.TrimSpace(parts[2]),
			Date:        strings.TrimSpace(parts[3]),
			User:        strings.TrimSpace(parts[4]),
			Cleanup:     strings.TrimSpace(parts[5]),
			Description: strings.TrimSpace(parts[6]),
		}

		if len(parts) >= 8 {
			snap.UserData = strings.TrimSpace(parts[7])
		}

		snapshots = append(snapshots, snap)
	}
	return snapshots
}

func parseDiffList(input string) []DiffEntry {
	lines := strings.Split(input, "\n")
	var entries []DiffEntry
	for _, line := range lines {
		// Status line format: "c..... /path/to/file"
		// Only care about lines with content
		if len(line) < 3 {
			continue
		}
		
		// The first character usually indicates the change type (+, -, c, .)
		// Sometimes there's whitespace.
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		
		// Snapper status output:
		// +..... /file/path
		// -..... /file/path
		// c..... /file/path
		
		action := string(line[0])
		// Basic mapping
		if action == "c" { action = "M" } // Modified
		
		// Path starts after the status columns
		// Status columns are fixed width (usually space separated from path)
		// We can just take the last part or everything after the first whitespace block
		
		// Simple approach: split by first space, but file paths can have spaces.
		// Snapper status usually puts path at the end.
		// However, let's use the provided logic which seemed to work for tree view
		// Previous logic: entries = append(entries, DiffEntry{Action: string(line[0]), Path: strings.TrimSpace(line[7:])})
		// We'll stick to a slightly more robust one
		
		path := line[strings.Index(line, " ")+1:]
		path = strings.TrimSpace(path)
		
		entries = append(entries, DiffEntry{Action: action, Path: path})
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
			// Normalize separator
			line = strings.ReplaceAll(line, "│", "|")
			parts := strings.Split(line, "|")
			if len(parts) == 2 {
				res[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
		json.NewEncoder(w).Encode(res)
	})

	http.HandleFunc("/api/snapshots", func(w http.ResponseWriter, r *http.Request) {
		config := r.URL.Query().Get("config")
		// Force list columns to ensure we get userdata
		cmd := exec.Command("snapper", "-c", config, "list", "--columns", "id,type,pre-id,date,user,cleanup,description,userdata")
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
		
		// Construct args: snapper -c config undochange 1..2 -- "path1" "path2"
		args := []string{" -c", req.Config, "undochange", req.Range, "--"}
		args = append(args, req.Paths...)
		
		err := exec.Command("snapper", args...).Run()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.WriteHeader(200)
	})

	http.HandleFunc("/api/rollback", func(w http.ResponseWriter, r *http.Request) {
		config := r.URL.Query().Get("config")
		id := r.URL.Query().Get("id")
		desc := fmt.Sprintf("Rollback to snapshot %s via WebUI", id)
		
		// Rollback usually requires root config
		cmd := exec.Command("snapper", "-c", config, "rollback", "-d", desc, id)
		err := cmd.Run()
		if err != nil {
			http.Error(w, "Rollback failed: "+err.Error(), 500)
			return
		}
		w.WriteHeader(200)
	})

	http.HandleFunc("/api/create", func(w http.ResponseWriter, r *http.Request) {
		config := r.URL.Query().Get("config")
		desc := r.URL.Query().Get("description")
		userdata := r.URL.Query().Get("userdata")
		args := []string{" -c", config, "create", "--description", desc}
		if userdata != "" {
			args = append(args, "--userdata", userdata)
		}
		// Print allows use to use default type (usually 'single')
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
