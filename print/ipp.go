package print

import (
	"bytes"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	goipp "github.com/phin1x/go-ipp"
)

// parseIPPURI splits an IPP URI into host, port, and HTTP path for sending requests.
func parseIPPURI(ippURI string) (host string, port int, httpURL string, err error) {
	// Normalize scheme
	normalized := ippURI
	if strings.HasPrefix(normalized, "ipps://") {
		normalized = strings.Replace(normalized, "ipps://", "https://", 1)
	} else {
		normalized = strings.Replace(normalized, "ipp://", "http://", 1)
	}
	if !strings.HasPrefix(normalized, "http") {
		normalized = "http://" + normalized
	}

	u, parseErr := url.Parse(normalized)
	if parseErr != nil {
		return "", 0, "", parseErr
	}
	host = u.Hostname()
	path := u.Path
	if path == "" {
		path = "/ipp/print"
	}
	portStr := u.Port()
	if portStr == "" {
		port = 631
	} else {
		p, e := strconv.Atoi(portStr)
		if e != nil {
			return "", 0, "", fmt.Errorf("invalid port: %w", e)
		}
		port = p
	}
	httpURL = fmt.Sprintf("%s://%s:%d%s", u.Scheme, host, port, path)
	return host, port, httpURL, nil
}

// PrintFile sends a document to an IPP network printer.
// ippURI example: "ipp://192.168.1.50:631/ipp/print"
// Returns the IPP job ID as string.
func PrintFile(ippURI, fileName, contentType string, data []byte, copies int) (string, error) {
	if !strings.HasPrefix(ippURI, "ipp://") && !strings.HasPrefix(ippURI, "ipps://") {
		ippURI = "ipp://" + ippURI
	}

	host, port, httpURL, err := parseIPPURI(ippURI)
	if err != nil {
		return "", fmt.Errorf("parse IPP URI %q: %w", ippURI, err)
	}

	client := goipp.NewIPPClient(host, port, "cloudprint", "", false)

	if copies < 1 {
		copies = 1
	}

	jobAttrs := map[string]interface{}{
		goipp.AttributeCopies: copies,
	}

	doc := goipp.Document{
		Document: bytes.NewReader(data),
		Size:     len(data),
		Name:     fileName,
		MimeType: contentType,
	}

	// Build a Print-Job request manually so we can send to the full URI path
	req := goipp.NewRequest(goipp.OperationPrintJob, 1)
	req.OperationAttributes[goipp.AttributePrinterURI] = ippURI
	req.OperationAttributes[goipp.AttributeRequestingUserName] = "cloudprint"
	req.OperationAttributes[goipp.AttributeJobName] = fileName
	req.OperationAttributes[goipp.AttributeDocumentFormat] = contentType
	req.OperationAttributes[goipp.AttributeCopies] = copies

	for k, v := range jobAttrs {
		req.JobAttributes[k] = v
	}

	req.File = doc.Document
	req.FileSize = doc.Size

	resp, err := client.SendRequest(httpURL, req, nil)
	if err != nil {
		return "", fmt.Errorf("IPP print-job: %w", err)
	}
	if err := resp.CheckForErrors(); err != nil {
		return "", fmt.Errorf("IPP error: %w", err)
	}

	jobID := 0
	if len(resp.JobAttributes) > 0 {
		if attrs, ok := resp.JobAttributes[0][goipp.AttributeJobID]; ok && len(attrs) > 0 {
			if id, ok2 := attrs[0].Value.(int); ok2 {
				jobID = id
			}
		}
	}
	return strconv.Itoa(jobID), nil
}

// PrintFileCUPS uses the system lp command (Linux only) to print via CUPS.
func PrintFileCUPS(printerName string, data []byte, copies int) (string, error) {
	if runtime.GOOS == "windows" {
		return "", fmt.Errorf("CUPS not available on Windows")
	}
	if copies < 1 {
		copies = 1
	}
	tmpFile, err := os.CreateTemp("", "cloudprint-*.pdf")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return "", err
	}
	tmpFile.Close()

	args := []string{"-d", printerName, "-n", strconv.Itoa(copies), tmpFile.Name()}
	out, err := exec.Command("lp", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("lp: %w — %s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// ProbeIPP checks if a printer is reachable and returns its reported name.
func ProbeIPP(ippURI string) (name string, reachable bool) {
	if ippURI == "" {
		return "", false
	}
	if !strings.HasPrefix(ippURI, "ipp://") && !strings.HasPrefix(ippURI, "ipps://") {
		ippURI = "ipp://" + ippURI
	}

	host, port, httpURL, err := parseIPPURI(ippURI)
	if err != nil {
		return "", false
	}

	// Quick TCP check with short timeout — avoids hanging on unreachable hosts
	conn, dialErr := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), 3*time.Second)
	if dialErr != nil {
		return "", false
	}
	conn.Close()

	client := goipp.NewIPPClient(host, port, "cloudprint", "", false)

	req := goipp.NewRequest(goipp.OperationGetPrinterAttributes, 1)
	req.OperationAttributes[goipp.AttributePrinterURI] = ippURI
	req.OperationAttributes[goipp.AttributeRequestingUserName] = "cloudprint"
	req.OperationAttributes[goipp.AttributeRequestedAttributes] = []string{goipp.AttributePrinterName}

	resp, err := client.SendRequest(httpURL, req, nil)
	if err != nil || resp.CheckForErrors() != nil {
		return "", false
	}

	if len(resp.PrinterAttributes) > 0 {
		if attrs, ok := resp.PrinterAttributes[0][goipp.AttributePrinterName]; ok && len(attrs) > 0 {
			if s, ok2 := attrs[0].Value.(string); ok2 {
				name = s
			}
		}
	}
	return name, true
}
