package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"runtime"
	"time"
)

type Client struct {
	baseURL string
	agentID string
	apiKey  string
	http    *http.Client
}

func New(baseURL, agentID, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		agentID: agentID,
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) do(method, path string, body interface{}) (*http.Response, error) {
	var buf io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		buf = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, c.baseURL+"/"+path, buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	return c.http.Do(req)
}

func (c *Client) decodeResponse(res *http.Response, out interface{}) error {
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("HTTP %d: %s", res.StatusCode, string(body))
	}
	if out != nil {
		return json.NewDecoder(res.Body).Decode(out)
	}
	return nil
}

// RegisterRequest exchanges an install token for a permanent API key.
type RegisterRequest struct {
	InstallToken string `json:"install_token"`
	AgentName    string `json:"agent_name"`
	Platform     string `json:"platform"`
	Version      string `json:"version"`
	Hostname     string `json:"hostname"`
	IPAddress    string `json:"ip_address"`
}

type RegisterResponse struct {
	AgentID string `json:"agent_id"`
	APIKey  string `json:"api_key"`
}

func Register(baseURL, installToken, name, version string) (*RegisterResponse, error) {
	hostname, _ := getHostname()
	ip, _ := getLocalIP()
	platform := runtime.GOOS + "_" + runtime.GOARCH

	body := RegisterRequest{
		InstallToken: installToken,
		AgentName:    name,
		Platform:     platform,
		Version:      version,
		Hostname:     hostname,
		IPAddress:    ip,
	}
	data, _ := json.Marshal(body)
	res, err := http.Post(baseURL+"/registerAgent", "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		b, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("registration failed (HTTP %d): %s", res.StatusCode, b)
	}
	var out RegisterResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PrinterStatus holds printer reachability for heartbeat.
type PrinterStatus struct {
	CloudPrinterID string `json:"cloud_printer_id"`
	Reachable      bool   `json:"reachable"`
}

// HeartbeatRequest is sent by the agent every heartbeat interval.
type HeartbeatRequest struct {
	Version         string          `json:"version,omitempty"`
	Hostname        string          `json:"hostname,omitempty"`
	IPAddress       string          `json:"ip_address,omitempty"`
	PrinterStatuses []PrinterStatus `json:"printer_statuses,omitempty"`
}

// HeartbeatConfig holds server-side tuning returned in heartbeat responses.
type HeartbeatConfig struct {
	PollIntervalSeconds      int `json:"poll_interval_seconds"`
	HeartbeatIntervalSeconds int `json:"heartbeat_interval_seconds"`
}

// HeartbeatResponse is returned by agentHeartbeat.
type HeartbeatResponse struct {
	OK     bool            `json:"ok"`
	Config HeartbeatConfig `json:"config"`
}

func (c *Client) Heartbeat(req HeartbeatRequest) (*HeartbeatResponse, error) {
	res, err := c.do("POST", "agentHeartbeat", req)
	if err != nil {
		return nil, err
	}
	var wrapper struct {
		OK     bool            `json:"ok"`
		Config HeartbeatConfig `json:"config"`
	}
	if err := c.decodeResponse(res, &wrapper); err != nil {
		return nil, err
	}
	return &HeartbeatResponse{OK: wrapper.OK, Config: wrapper.Config}, nil
}

// Job represents a pending print job from the cloud.
type Job struct {
	JobID       string `json:"job_id"`
	PrinterID   string `json:"printer_id"`
	IPPURI      string `json:"ipp_uri"`
	PrinterName string `json:"printer_name"`
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	DownloadURL string `json:"download_url"`
	Copies      int    `json:"copies"`
	Color       bool   `json:"color"`
	Duplex      bool   `json:"duplex"`
}

// PollResponse wraps the list of pending jobs.
type PollResponse struct {
	Jobs []Job `json:"jobs"`
}

// PollJobs fetches pending jobs for this agent's printers.
func (c *Client) PollJobs() ([]Job, error) {
	res, err := c.do("GET", "agentPollJobs", nil)
	if err != nil {
		return nil, err
	}
	var out PollResponse
	if err := c.decodeResponse(res, &out); err != nil {
		return nil, err
	}
	return out.Jobs, nil
}

// DownloadFile downloads the file for a job using a one-time token URL.
func (c *Client) DownloadFile(downloadURL string) ([]byte, string, error) {
	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return nil, "", err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		b, _ := io.ReadAll(res.Body)
		return nil, "", fmt.Errorf("download failed (HTTP %d): %s", res.StatusCode, b)
	}
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, "", err
	}
	ct := res.Header.Get("Content-Type")
	if ct == "" {
		ct = "application/pdf"
	}
	return data, ct, nil
}

// UpdateJobRequest reports job completion or failure to the cloud.
type UpdateJobRequest struct {
	JobID        string `json:"job_id"`
	Status       string `json:"status"`
	AgentJobID   string `json:"agent_job_id,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
	Pages        int    `json:"pages,omitempty"`
}

// UpdateJobStatus reports job completion or failure to the cloud.
func (c *Client) UpdateJobStatus(req UpdateJobRequest) error {
	res, err := c.do("POST", "agentUpdateJobStatus", req)
	if err != nil {
		return err
	}
	return c.decodeResponse(res, nil)
}

// DiscoveredPrinter holds data about a printer found in the LAN.
type DiscoveredPrinter struct {
	LocalName    string                 `json:"local_name"`
	IPPURI       string                 `json:"ipp_uri"`
	Model        string                 `json:"model,omitempty"`
	Location     string                 `json:"location,omitempty"`
	Reachable    bool                   `json:"reachable"`
	Capabilities map[string]interface{} `json:"capabilities,omitempty"`
}

// ReportPrintersRequest wraps discovered printers for reporting.
type ReportPrintersRequest struct {
	Printers []DiscoveredPrinter `json:"printers"`
}

// ReportPrinters reports discovered printers to the cloud.
func (c *Client) ReportPrinters(printers []DiscoveredPrinter) error {
	res, err := c.do("POST", "agentReportPrinters", ReportPrintersRequest{Printers: printers})
	if err != nil {
		return err
	}
	return c.decodeResponse(res, nil)
}

// helpers

func getHostname() (string, error) {
	return os.Hostname()
}

func getLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String(), nil
			}
		}
	}
	return "", nil
}
