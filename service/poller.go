package service

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/cloudprint/cloudprint-agent/api"
	"github.com/cloudprint/cloudprint-agent/config"
	printpkg "github.com/cloudprint/cloudprint-agent/print"
)

// Version is the current agent version.
const Version = "1.0.0"

// Poller manages the main poll/heartbeat loop.
type Poller struct {
	cfg    *config.Config
	client *api.Client
	logger *slog.Logger
}

// NewPoller creates a Poller from the given config.
func NewPoller(cfg *config.Config) *Poller {
	client := api.New(cfg.APIURL, cfg.AgentID, cfg.APIKey)
	return &Poller{
		cfg:    cfg,
		client: client,
		logger: slog.Default(),
	}
}

// Run starts the heartbeat and polling loops until ctx is cancelled.
func (p *Poller) Run(ctx context.Context) error {
	p.logger.Info("cloudprint-agent starting", "version", Version, "agent", p.cfg.AgentName)

	pollTicker := time.NewTicker(time.Duration(p.cfg.PollInterval) * time.Second)
	heartbeatTicker := time.NewTicker(time.Duration(p.cfg.HeartbeatInterval) * time.Second)
	defer pollTicker.Stop()
	defer heartbeatTicker.Stop()

	// Initial heartbeat
	p.sendHeartbeat()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("agent stopped")
			return ctx.Err()
		case <-heartbeatTicker.C:
			p.sendHeartbeat()
		case <-pollTicker.C:
			p.pollAndPrint()
		}
	}
}

func (p *Poller) sendHeartbeat() {
	hostname, _ := os.Hostname()
	ip, _ := getLocalIP()

	statuses := make([]api.PrinterStatus, 0, len(p.cfg.Printers))
	for _, pr := range p.cfg.Printers {
		_, reachable := printpkg.ProbeIPP(pr.IPPURI)
		statuses = append(statuses, api.PrinterStatus{
			CloudPrinterID: pr.CloudPrinterID,
			Reachable:      reachable,
		})
	}

	resp, err := p.client.Heartbeat(api.HeartbeatRequest{
		Version:         Version,
		Hostname:        hostname,
		IPAddress:       ip,
		PrinterStatuses: statuses,
	})
	if err != nil {
		p.logger.Warn("heartbeat failed", "err", err)
		return
	}

	// Update poll interval from server config
	if resp.Config.PollIntervalSeconds > 0 {
		p.cfg.PollInterval = resp.Config.PollIntervalSeconds
	}
	p.logger.Debug("heartbeat ok")
}

func (p *Poller) pollAndPrint() {
	jobs, err := p.client.PollJobs()
	if err != nil {
		p.logger.Warn("poll failed", "err", err)
		return
	}
	if len(jobs) == 0 {
		return
	}
	p.logger.Info("jobs received", "count", len(jobs))

	for _, job := range jobs {
		go p.processJob(job)
	}
}

func (p *Poller) processJob(job api.Job) {
	p.logger.Info("processing job", "job_id", job.JobID, "file", job.FileName, "printer", job.PrinterName)

	// Download file
	data, contentType, err := p.client.DownloadFile(job.DownloadURL)
	if err != nil {
		p.logger.Error("download failed", "job_id", job.JobID, "err", err)
		_ = p.client.UpdateJobStatus(api.UpdateJobRequest{
			JobID:        job.JobID,
			Status:       "failed",
			ErrorMessage: fmt.Sprintf("download failed: %s", err),
		})
		return
	}
	p.logger.Info("file downloaded", "job_id", job.JobID, "size", len(data), "content_type", contentType)

	// Find local printer config
	ippURI := job.IPPURI
	method := "ipp"
	copies := job.Copies
	if copies == 0 {
		copies = 1
	}

	for _, pr := range p.cfg.Printers {
		if pr.CloudPrinterID == job.PrinterID {
			if pr.IPPURI != "" {
				ippURI = pr.IPPURI
			}
			if pr.Method != "" {
				method = pr.Method
			}
			break
		}
	}

	if ippURI == "" {
		p.logger.Error("no IPP URI for printer", "printer_id", job.PrinterID)
		_ = p.client.UpdateJobStatus(api.UpdateJobRequest{
			JobID:        job.JobID,
			Status:       "failed",
			ErrorMessage: "no IPP URI configured for printer",
		})
		return
	}

	var jobID string
	if method == "cups" && runtime.GOOS == "linux" {
		// Extract CUPS queue name from URI
		printerName := ippURI
		if idx := strings.LastIndex(ippURI, "/"); idx >= 0 {
			printerName = ippURI[idx+1:]
		}
		jobID, err = printpkg.PrintFileCUPS(printerName, data, copies)
	} else {
		jobID, err = printpkg.PrintFile(ippURI, job.FileName, contentType, data, copies)
	}

	if err != nil {
		p.logger.Error("print failed", "job_id", job.JobID, "err", err)
		_ = p.client.UpdateJobStatus(api.UpdateJobRequest{
			JobID:        job.JobID,
			Status:       "failed",
			ErrorMessage: err.Error(),
		})
		return
	}

	p.logger.Info("print submitted", "job_id", job.JobID, "local_job_id", jobID)
	_ = p.client.UpdateJobStatus(api.UpdateJobRequest{
		JobID:      job.JobID,
		Status:     "printed",
		AgentJobID: jobID,
	})
}

func getLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
			return ipNet.IP.String(), nil
		}
	}
	return "", nil
}
