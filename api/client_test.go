package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHeartbeat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/agentHeartbeat" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer sk-agent-test" {
			t.Errorf("missing or wrong Authorization header")
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":       true,
			"agent_id": "agt-123",
			"config": map[string]interface{}{
				"poll_interval_seconds":      5,
				"heartbeat_interval_seconds": 30,
			},
		})
	}))
	defer srv.Close()

	client := New(srv.URL, "agt-123", "sk-agent-test")
	resp, err := client.Heartbeat(HeartbeatRequest{Version: "1.0.0"})
	if err != nil {
		t.Fatalf("Heartbeat failed: %v", err)
	}
	if !resp.OK {
		t.Error("expected ok=true")
	}
	if resp.Config.PollIntervalSeconds != 5 {
		t.Errorf("PollIntervalSeconds = %d, want 5", resp.Config.PollIntervalSeconds)
	}
}

func TestPollJobsEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"jobs": []interface{}{}})
	}))
	defer srv.Close()

	client := New(srv.URL, "agt-123", "sk-agent-test")
	jobs, err := client.PollJobs()
	if err != nil {
		t.Fatalf("PollJobs failed: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(jobs))
	}
}

func TestPollJobsWithJob(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"jobs": []map[string]interface{}{
				{
					"job_id":       "job-abc",
					"printer_id":   "printer-1",
					"ipp_uri":      "ipp://192.168.1.50:631/ipp/print",
					"file_name":    "test.pdf",
					"content_type": "application/pdf",
					"download_url": "https://api.example.com/download?token=xyz",
					"copies":       1,
				},
			},
		})
	}))
	defer srv.Close()

	client := New(srv.URL, "agt-123", "sk-agent-test")
	jobs, err := client.PollJobs()
	if err != nil {
		t.Fatalf("PollJobs failed: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].JobID != "job-abc" {
		t.Errorf("JobID = %q, want job-abc", jobs[0].JobID)
	}
	if jobs[0].FileName != "test.pdf" {
		t.Errorf("FileName = %q, want test.pdf", jobs[0].FileName)
	}
}

func TestUpdateJobStatus(t *testing.T) {
	var received UpdateJobRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	}))
	defer srv.Close()

	client := New(srv.URL, "agt-123", "sk-agent-test")
	err := client.UpdateJobStatus(UpdateJobRequest{
		JobID:      "job-abc",
		Status:     "printed",
		AgentJobID: "local-42",
		Pages:      3,
	})
	if err != nil {
		t.Fatalf("UpdateJobStatus failed: %v", err)
	}
	if received.JobID != "job-abc" {
		t.Errorf("JobID = %q", received.JobID)
	}
	if received.Status != "printed" {
		t.Errorf("Status = %q", received.Status)
	}
	if received.Pages != 3 {
		t.Errorf("Pages = %d", received.Pages)
	}
}

func TestHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	client := New(srv.URL, "agt-123", "wrong-key")
	_, err := client.PollJobs()
	if err == nil {
		t.Error("expected error for 401 response")
	}
}
