package print

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	goipp "github.com/phin1x/go-ipp"
)

// parseIPPURI splits an IPP URI into host, port, and resource path.
func parseIPPURI(ippURI string) (host string, port int, path string, err error) {
	u, parseErr := url.Parse(ippURI)
	if parseErr != nil {
		return "", 0, "", parseErr
	}
	host = u.Hostname()
	path = u.Path
	if path == "" {
		path = "/ipp/print"
	}
	portStr := u.Port()
	if portStr == "" {
		port = 631
	} else {
		port64, e := strconv.ParseInt(portStr, 10, 32)
		if e != nil {
			return "", 0, "", fmt.Errorf("invalid port in IPP URI: %w", e)
		}
		port = int(port64)
	}
	return host, port, path, nil
}

// PrintFile sends a document to an IPP printer.
// ippURI example: "ipp://192.168.1.50:631/ipp/print"
// Returns the IPP job ID string.
func PrintFile(ippURI, fileName, contentType string, data []byte, copies int) (string, error) {
	// Normalize URI scheme
	uri := ippURI
	if !strings.HasPrefix(uri, "ipp://") && !strings.HasPrefix(uri, "ipps://") {
		uri = "ipp://" + uri
	}

	host, port, printerPath, err := parseIPPURI(uri)
	if err != nil {
		return "", fmt.Errorf("parse IPP URI: %w", err)
	}

	client := goipp.NewIppClient(host, port)

	req := &goipp.IppRequest{
		ProtocolVersionMajor: 1,
		ProtocolVersionMinor: 1,
		Operation:            goipp.OperationPrintJob,
		RequestId:            1,
	}
	req.OperationAttributes = map[string][]goipp.Attribute{
		"attributes-charset":          {{Value: goipp.String("utf-8"), Tag: goipp.TagCharset}},
		"attributes-natural-language": {{Value: goipp.String("en"), Tag: goipp.TagLanguage}},
		"printer-uri":                 {{Value: goipp.String(uri), Tag: goipp.TagURI}},
		"requesting-user-name":        {{Value: goipp.String("cloudprint"), Tag: goipp.TagName}},
		"job-name":                    {{Value: goipp.String(fileName), Tag: goipp.TagName}},
		"document-format":             {{Value: goipp.String(contentType), Tag: goipp.TagMimeType}},
	}
	req.JobAttributes = map[string][]goipp.Attribute{}
	if copies > 1 {
		req.JobAttributes["copies"] = []goipp.Attribute{{Value: goipp.Integer(copies), Tag: goipp.TagInteger}}
	}
	req.File = data
	req.FileSize = len(data)

	resp, err := client.SendRequest(printerPath, req)
	if err != nil {
		return "", fmt.Errorf("IPP print-job: %w", err)
	}
	if resp.StatusCode != goipp.StatusOk && resp.StatusCode != goipp.StatusOkSubstitutedValues {
		return "", fmt.Errorf("IPP error: %s (code %d)", resp.StatusMessage, resp.StatusCode)
	}

	jobID := ""
	if attrs, ok := resp.JobAttributes["job-id"]; ok && len(attrs) > 0 {
		jobID = fmt.Sprintf("%v", attrs[0].Value)
	}
	return jobID, nil
}

// PrintFileCUPS uses the system lp command (Linux only) to print via CUPS.
// printerName is the CUPS queue name.
func PrintFileCUPS(printerName string, data []byte, copies int) (string, error) {
	if runtime.GOOS == "windows" {
		return "", fmt.Errorf("CUPS not available on Windows")
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

	args := []string{"-d", printerName, "-n", fmt.Sprintf("%d", copies), tmpFile.Name()}
	out, err := exec.Command("lp", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("lp error: %w — %s", err, string(out))
	}
	// lp outputs: "request id is printer-N (1 file(s))"
	return strings.TrimSpace(string(out)), nil
}

// ProbeIPP checks if an IPP printer is reachable and returns its name.
func ProbeIPP(ippURI string) (name string, reachable bool) {
	host, port, printerPath, err := parseIPPURI(ippURI)
	if err != nil {
		return "", false
	}

	client := goipp.NewIppClient(host, port)

	req := &goipp.IppRequest{
		ProtocolVersionMajor: 1,
		ProtocolVersionMinor: 1,
		Operation:            goipp.OperationGetPrinterAttributes,
		RequestId:            1,
	}
	req.OperationAttributes = map[string][]goipp.Attribute{
		"attributes-charset":          {{Value: goipp.String("utf-8"), Tag: goipp.TagCharset}},
		"attributes-natural-language": {{Value: goipp.String("en"), Tag: goipp.TagLanguage}},
		"printer-uri":                 {{Value: goipp.String(ippURI), Tag: goipp.TagURI}},
		"requested-attributes":        {{Value: goipp.String("printer-name"), Tag: goipp.TagKeyword}},
	}
	req.JobAttributes = map[string][]goipp.Attribute{}

	resp, err := client.SendRequest(printerPath, req)
	if err != nil || resp.StatusCode != goipp.StatusOk {
		return "", false
	}
	if attrs, ok := resp.PrinterAttributes["printer-name"]; ok && len(attrs) > 0 {
		name = fmt.Sprintf("%v", attrs[0].Value)
	}
	return name, true
}
