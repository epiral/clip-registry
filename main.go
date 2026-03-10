package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: registry <command>")
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "list":
		err = cmdList()
	case "list-servers":
		err = cmdListServers()
	case "add-server":
		err = cmdAddServer()
	case "remove-server":
		err = cmdRemoveServer()
	case "generate-bookmark":
		err = cmdGenerateBookmark()
	default:
		err = fmt.Errorf("unknown command: %s", os.Args[1])
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

// --- Data types ---

type Server struct {
	Name      string `json:"name"`
	ServerURL string `json:"server_url"`
	Token     string `json:"token"`
}

type Clip struct {
	ClipID   string   `json:"clipId"`
	Name     string   `json:"name"`
	Desc     string   `json:"desc,omitempty"`
	Commands []string `json:"commands,omitempty"`
	HasWeb   bool     `json:"hasWeb,omitempty"`
	Server   string   `json:"server,omitempty"`
	URL      string   `json:"server_url,omitempty"`
}

// --- Paths ---

func dataDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "data"
	}
	return filepath.Join(filepath.Dir(filepath.Dir(exe)), "data")
}

func serversPath() string {
	return filepath.Join(dataDir(), "servers.json")
}

func loadServers() ([]Server, error) {
	raw, err := os.ReadFile(serversPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var servers []Server
	return servers, json.Unmarshal(raw, &servers)
}

func saveServers(servers []Server) error {
	os.MkdirAll(dataDir(), 0o755)
	raw, _ := json.MarshalIndent(servers, "", "  ")
	return os.WriteFile(serversPath(), raw, 0o644)
}

// --- HTTP helper ---

var httpClient = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	},
}

func rpcCall(serverURL, method, token string, body any) ([]byte, error) {
	payload, _ := json.Marshal(body)
	url := serverURL + "/" + method
	req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func readStdin() []byte {
	data, _ := io.ReadAll(os.Stdin)
	return data
}

// --- Commands ---

func cmdListServers() error {
	servers, err := loadServers()
	if err != nil {
		return err
	}

	type masked struct {
		Name      string `json:"name"`
		ServerURL string `json:"server_url"`
		TokenHint string `json:"token_hint"`
	}

	result := make([]masked, len(servers))
	for i, s := range servers {
		hint := ""
		if len(s.Token) >= 4 {
			hint = s.Token[len(s.Token)-4:]
		}
		result[i] = masked{Name: s.Name, ServerURL: s.ServerURL, TokenHint: hint}
	}

	out, _ := json.MarshalIndent(map[string]any{"servers": result}, "", "  ")
	fmt.Println(string(out))
	return nil
}

func cmdAddServer() error {
	var input Server
	if err := json.Unmarshal(readStdin(), &input); err != nil {
		return fmt.Errorf("invalid JSON input: %w", err)
	}
	if input.Name == "" || input.ServerURL == "" || input.Token == "" {
		return fmt.Errorf("missing required fields: name, server_url, token")
	}

	servers, _ := loadServers()
	for _, s := range servers {
		if s.Name == input.Name {
			return fmt.Errorf("server '%s' already exists, remove it first", input.Name)
		}
	}

	servers = append(servers, input)
	if err := saveServers(servers); err != nil {
		return err
	}

	fmt.Printf(`{"ok":true,"message":"server '%s' added"}%s`, input.Name, "\n")
	return nil
}

func cmdRemoveServer() error {
	var input struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(readStdin(), &input); err != nil {
		return fmt.Errorf("invalid JSON input: %w", err)
	}
	if input.Name == "" {
		return fmt.Errorf("missing required field: name")
	}

	servers, err := loadServers()
	if err != nil {
		return err
	}

	filtered := make([]Server, 0, len(servers))
	found := false
	for _, s := range servers {
		if s.Name == input.Name {
			found = true
			continue
		}
		filtered = append(filtered, s)
	}
	if !found {
		return fmt.Errorf("server '%s' not found", input.Name)
	}

	if err := saveServers(filtered); err != nil {
		return err
	}

	fmt.Printf(`{"ok":true,"message":"server '%s' removed"}%s`, input.Name, "\n")
	return nil
}

func cmdList() error {
	servers, err := loadServers()
	if err != nil {
		return err
	}
	if len(servers) == 0 {
		return fmt.Errorf("no servers configured, use add-server first")
	}

	// Optional filter by server name
	var input struct {
		Server string `json:"server"`
	}
	raw := readStdin()
	if len(raw) > 0 {
		json.Unmarshal(raw, &input)
	}

	if input.Server != "" {
		filtered := make([]Server, 0)
		for _, s := range servers {
			if s.Name == input.Server {
				filtered = append(filtered, s)
			}
		}
		if len(filtered) == 0 {
			return fmt.Errorf("server '%s' not found", input.Server)
		}
		servers = filtered
	}

	var allClips []Clip
	var errors []map[string]string

	for _, srv := range servers {
		resp, err := rpcCall(srv.ServerURL, "pinix.v1.AdminService/ListClips", srv.Token, map[string]any{})
		if err != nil {
			errors = append(errors, map[string]string{"server": srv.Name, "error": err.Error()})
			continue
		}

		var result struct {
			Clips []Clip `json:"clips"`
		}
		if err := json.Unmarshal(resp, &result); err != nil {
			errors = append(errors, map[string]string{"server": srv.Name, "error": "invalid response"})
			continue
		}

		for i := range result.Clips {
			result.Clips[i].Server = srv.Name
			result.Clips[i].URL = srv.ServerURL
		}
		allClips = append(allClips, result.Clips...)
	}

	if allClips == nil {
		allClips = []Clip{}
	}
	if errors == nil {
		errors = []map[string]string{}
	}

	out, _ := json.MarshalIndent(map[string]any{"clips": allClips, "errors": errors}, "", "  ")
	fmt.Println(string(out))
	return nil
}

func cmdGenerateBookmark() error {
	var input struct {
		Server string `json:"server"`
		ClipID string `json:"clip_id"`
	}
	if err := json.Unmarshal(readStdin(), &input); err != nil {
		return fmt.Errorf("invalid JSON input: %w", err)
	}
	if input.Server == "" || input.ClipID == "" {
		return fmt.Errorf("missing required fields: server, clip_id")
	}

	servers, err := loadServers()
	if err != nil {
		return err
	}

	var srv *Server
	for i := range servers {
		if servers[i].Name == input.Server {
			srv = &servers[i]
			break
		}
	}
	if srv == nil {
		return fmt.Errorf("server '%s' not found", input.Server)
	}

	// Generate token
	genResp, err := rpcCall(srv.ServerURL, "pinix.v1.AdminService/GenerateToken", srv.Token,
		map[string]string{"clipId": input.ClipID, "label": "registry-auto"})
	if err != nil {
		return fmt.Errorf("generate token: %w", err)
	}

	var tokenResult struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(genResp, &tokenResult); err != nil || tokenResult.Token == "" {
		return fmt.Errorf("generate token failed: %s", string(genResp))
	}

	// Get clip info
	infoResp, err := rpcCall(srv.ServerURL, "pinix.v1.ClipService/GetInfo", tokenResult.Token, map[string]any{})
	if err != nil {
		return fmt.Errorf("get info: %w", err)
	}

	var info struct {
		Name   string `json:"name"`
		HasWeb bool   `json:"hasWeb"`
	}
	json.Unmarshal(infoResp, &info)

	clipName := info.Name
	if clipName == "" {
		clipName = input.ClipID
	}

	bookmark := map[string]any{
		"name":       clipName,
		"server_url": srv.ServerURL,
		"token":      tokenResult.Token,
	}
	if info.HasWeb {
		bookmark["hasWeb"] = true
	}

	out, _ := json.MarshalIndent(bookmark, "", "  ")
	fmt.Println(string(out))
	return nil
}
