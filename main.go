package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"io/ioutil"
	"net/http"
	"os/exec"
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
	for i, line := range lines {
		if i < 2 || strings.TrimSpace(line) == "" { continue }
		line = strings.ReplaceAll(line, "│", "|")
		parts := strings.Split(line, "|")
		if len(parts) < 7 { continue }
		idStr := strings.TrimSpace(parts[0])
		id, err := strconv.Atoi(idStr)
		if err != nil { continue }
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
		if len(line) < 3 { continue }
		parts := strings.Fields(line)
		if len(parts) < 2 { continue }
		action := string(line[0])
		if action == "c" { action = "M" }
		pathIndex := strings.Index(line, "/")
		if pathIndex == -1 { continue }
		path := strings.TrimSpace(line[pathIndex:])
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
		files, err := ioutil.ReadDir("/etc/snapper/configs")
		var configs []string
		if err == nil {
			for _, f := range files { configs = append(configs, f.Name()) }
		} else {
			cmd := exec.Command("snapper", "list-configs")
			out, _ := cmd.Output()
			lines := strings.Split(string(out), "\n")
			for i, line := range lines {
				if i < 2 || line == "" { continue }
				line = strings.ReplaceAll(line, "│", " ")
				parts := strings.Fields(line)
				if len(parts) > 0 {
					cfg := parts[0]
					if cfg != "Config" && !strings.Contains(cfg, "──") { configs = append(configs, cfg) }
				}
			}
		}
		json.NewEncoder(w).Encode(configs)
	})

	http.HandleFunc("/api/get-config", func(w http.ResponseWriter, r *http.Request) {
		config := r.URL.Query().Get("config")
		cmd := exec.Command("snapper", "-c", config, "get-config")
		output, _ := cmd.CombinedOutput()
		lines := strings.Split(string(output), "\n")
		res := make(map[string]string)
		for _, line := range lines {
			sep := ""
			if strings.Contains(line, "│") { sep = "│" } else if strings.Contains(line, "|") { sep = "|" }
			if sep != "" {
				parts := strings.Split(line, sep)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					val := strings.TrimSpace(parts[1])
					if key != "" && key != "Key" && !strings.Contains(key, "──") { res[key] = val }
				}
			}
		}
		json.NewEncoder(w).Encode(res)
	})

	http.HandleFunc("/api/snapshots", func(w http.ResponseWriter, r *http.Request) {
		config := r.URL.Query().Get("config")
		cmd := exec.Command("snapper", "-c", config, "list", "--columns", "number,type,pre-number,date,user,cleanup,description,userdata")
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
		if r.Method != http.MethodPost { return }
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
		if userdata != "" { args = append(args, "--userdata", userdata) }
		if err := exec.Command("snapper", args...).Run(); err != nil {
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
