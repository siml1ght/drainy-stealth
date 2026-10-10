package uploadutil

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// UploadBufferSize is the copy buffer for streaming file bodies. A larger buffer
// reduces per-chunk overhead on fast links (~500 Mbit/s).
const UploadBufferSize = 4 * 1024 * 1024

// APIEndpoint is the PixelDrain upload endpoint.
const APIEndpoint = "https://pixeldrain.com/api/file"

// Result is the minimal PixelDrain upload response.
type Result struct {
	ID      string `json:"id"`
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ProgressFunc receives bytes written so far and the total body size.
type ProgressFunc func(written, total int64)

// progressWriter counts bytes written to the request body and reports progress.
type progressWriter struct {
	w        io.Writer
	written  int64
	total    int64
	progress ProgressFunc
	last     time.Time
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n, err := p.w.Write(b)
	p.written += int64(n)
	if p.progress != nil && (time.Since(p.last) > 100*time.Millisecond || p.written >= p.total) {
		p.last = time.Now()
		p.progress(p.written, p.total)
	}
	return n, err
}

// UploadFile streams filePath to PixelDrain as multipart/form-data using a
// 4 MiB copy buffer. apiKey is sent as Basic auth (empty user, key as password)
// when non-empty; otherwise the upload is anonymous.
func UploadFile(filePath, fileName, apiKey string, progress ProgressFunc) (string, error) {
	if filePath == "" {
		return "", errors.New("file path can not be empty")
	}
	if fileName == "" {
		fileName = filepath.Base(filePath)
	}

	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", errors.New("path is a directory")
	}

	// Build the multipart envelope; the file part is streamed from disk.
	// Multipart envelope: header part before the file, closing boundary after it.
	var head bytes.Buffer
	mw := multipart.NewWriter(&head)
	if _, err := mw.CreateFormFile("file", fileName); err != nil {
		return "", err
	}
	headBytes := head.Bytes()
	// CreateFormFile wrote the part header; the closing boundary goes last.
	tailBytes := []byte("\r\n--" + mw.Boundary() + "--\r\n")

	total := int64(len(headBytes)) + info.Size() + int64(len(tailBytes))

	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		if _, err := pw.Write(headBytes); err != nil {
			return
		}
		counted := &progressWriter{w: pw, total: total, progress: progress, written: int64(len(headBytes))}
		buf := make([]byte, UploadBufferSize)
		if _, err := io.CopyBuffer(counted, f, buf); err != nil {
			pw.CloseWithError(err)
			return
		}
		_, _ = pw.Write(tailBytes)
	}()

	req, err := http.NewRequest(http.MethodPost, APIEndpoint+"?anonymous="+boolStr(apiKey == ""), pr)
	if err != nil {
		return "", err
	}
	req.ContentLength = total
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if apiKey != "" {
		req.SetBasicAuth("", apiKey)
	}

	resp, err := uploadClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var res Result
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", fmt.Errorf("bad response (HTTP %d): %w", resp.StatusCode, err)
	}
	if resp.StatusCode >= 400 || !res.Success || res.ID == "" {
		msg := res.Message
		if msg == "" {
			msg = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		return "", errors.New(msg)
	}
	return BaseURL + "u/" + res.ID, nil
}

// uploadClient has no overall timeout, so large files can take as long as they need
// to transfer. It does bound the connect and the wait for the server's response
// headers, so a dead connection cannot hang the upload forever.
var uploadClient = &http.Client{
	Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 10 * time.Minute, // server processing after the body is sent
		ExpectContinueTimeout: 1 * time.Second,
	},
}

// BaseURL is the public PixelDrain site root used to build share links.
const BaseURL = "https://pixeldrain.com/"

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
