package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
)

type repoHoverTSServerClient struct {
	runtime   repoHoverRuntime
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	stderrBuf bytes.Buffer

	writeMu  sync.Mutex
	stateMu  sync.Mutex
	pending  map[int64]chan tsServerResponseEnvelope
	openDocs map[string]string
	nextID   atomic.Int64
	done     chan struct{}
}

type tsServerRequestEnvelope struct {
	Seq       int64  `json:"seq"`
	Type      string `json:"type"`
	Command   string `json:"command"`
	Arguments any    `json:"arguments,omitempty"`
}

type tsServerResponseEnvelope struct {
	RequestSeq int64           `json:"request_seq"`
	Success    bool            `json:"success"`
	Command    string          `json:"command"`
	Message    string          `json:"message,omitempty"`
	Body       json.RawMessage `json:"body,omitempty"`
}

type tsServerEventEnvelope struct {
	Type  string `json:"type"`
	Event string `json:"event"`
}

type tsServerLocation struct {
	Line   int `json:"line"`
	Offset int `json:"offset"`
}

type tsServerDisplayPart struct {
	Text string `json:"text"`
	Kind string `json:"kind"`
}

type tsServerTag struct {
	Name string `json:"name"`
	Text any    `json:"text"`
}

type tsServerQuickInfoBody struct {
	Start         tsServerLocation `json:"start"`
	End           tsServerLocation `json:"end"`
	DisplayString string           `json:"displayString"`
	Documentation any              `json:"documentation"`
	Tags          []tsServerTag    `json:"tags"`
}

type tsServerFileSpan struct {
	File  string           `json:"file"`
	Start tsServerLocation `json:"start"`
	End   tsServerLocation `json:"end"`
}

func newTSServerHoverBackend(ctx context.Context, runtime repoHoverRuntime) (repoHoverBackend, error) {
	backendCtx, _ := context.WithCancel(ctx)
	cmd := exec.CommandContext(backendCtx, runtime.command, runtime.args...)
	cmd.Dir = runtime.rootPath

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("create %s stdout: %w", runtime.provider, err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("create %s stdin: %w", runtime.provider, err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("create %s stderr: %w", runtime.provider, err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", runtime.provider, err)
	}

	client := &repoHoverTSServerClient{
		runtime:  runtime,
		cmd:      cmd,
		stdin:    stdin,
		stdout:   stdout,
		pending:  make(map[int64]chan tsServerResponseEnvelope),
		openDocs: make(map[string]string),
		done:     make(chan struct{}),
	}
	go func() {
		_, _ = io.Copy(&client.stderrBuf, stderr)
	}()
	go client.readLoop()

	return client, nil
}

func (c *repoHoverTSServerClient) Hover(ctx context.Context, request repoHoverLSPRequest) (RepoFileHoverResponse, error) {
	if !c.Alive() {
		return RepoFileHoverResponse{}, errors.New("hover backend is not running")
	}
	if err := c.syncDocument(request.filePath, request.content); err != nil {
		return RepoFileHoverResponse{}, err
	}

	result, err := c.request(ctx, "quickinfo", map[string]any{
		"file":   request.filePath,
		"line":   request.line + 1,
		"offset": request.character + 1,
	})
	if err != nil {
		return RepoFileHoverResponse{}, err
	}
	if len(result) == 0 || bytes.Equal(result, []byte("null")) {
		return RepoFileHoverResponse{
			Supported: true,
			Available: true,
			Language:  request.language,
			Provider:  request.provider,
		}, nil
	}

	var quickInfo tsServerQuickInfoBody
	if err := json.Unmarshal(result, &quickInfo); err != nil {
		return RepoFileHoverResponse{}, err
	}

	documentation := strings.TrimSpace(renderTSServerDocumentation(quickInfo.Documentation))
	tagDocumentation := renderTSServerTags(quickInfo.Tags)
	if tagDocumentation != "" {
		if documentation != "" {
			documentation += "\n\n"
		}
		documentation += tagDocumentation
	}

	response := RepoFileHoverResponse{
		Supported:         true,
		Available:         true,
		Found:             quickInfo.DisplayString != "" || documentation != "",
		Language:          request.language,
		Provider:          request.provider,
		Header:            strings.TrimSpace(quickInfo.DisplayString),
		Documentation:     documentation,
		DocumentationKind: "plaintext",
		Source:            request.path,
	}
	if quickInfo.Start.Line > 0 && quickInfo.End.Line > 0 {
		response.Range = &RepoFileHoverRange{
			StartLine:      quickInfo.Start.Line - 1,
			StartCharacter: max(0, quickInfo.Start.Offset-1),
			EndLine:        quickInfo.End.Line - 1,
			EndCharacter:   max(0, quickInfo.End.Offset-1),
		}
	}
	return response, nil
}

func (c *repoHoverTSServerClient) Definition(ctx context.Context, request repoHoverLSPRequest) ([]repoFileDefinitionLocation, error) {
	if !c.Alive() {
		return nil, errors.New("hover backend is not running")
	}
	if err := c.syncDocument(request.filePath, request.content); err != nil {
		return nil, err
	}

	result, err := c.request(ctx, "definition", map[string]any{
		"file":   request.filePath,
		"line":   request.line + 1,
		"offset": request.character + 1,
	})
	if err != nil {
		return nil, err
	}
	if len(result) == 0 || bytes.Equal(result, []byte("null")) {
		return nil, nil
	}

	var spans []tsServerFileSpan
	if err := json.Unmarshal(result, &spans); err != nil {
		return nil, err
	}

	targets := make([]repoFileDefinitionLocation, 0, len(spans))
	for _, span := range spans {
		if strings.TrimSpace(span.File) == "" {
			continue
		}
		targets = append(targets, repoFileDefinitionLocation{
			filePath:       span.File,
			startLine:      max(0, span.Start.Line-1),
			startCharacter: max(0, span.Start.Offset-1),
			endLine:        max(0, span.End.Line-1),
			endCharacter:   max(0, span.End.Offset-1),
		})
	}
	return targets, nil
}

func (c *repoHoverTSServerClient) Close() error {
	if c == nil {
		return nil
	}
	select {
	case <-c.done:
	default:
		close(c.done)
	}
	_ = c.stdin.Close()
	if c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	_ = c.cmd.Wait()
	return nil
}

func (c *repoHoverTSServerClient) Alive() bool {
	if c == nil {
		return false
	}
	select {
	case <-c.done:
		return false
	default:
		return c.cmd.ProcessState == nil
	}
}

func (c *repoHoverTSServerClient) syncDocument(filePath, content string) error {
	c.stateMu.Lock()
	existing, isOpen := c.openDocs[filePath]
	c.stateMu.Unlock()

	if isOpen && existing == content {
		return nil
	}
	if isOpen {
		if err := c.notify("close", map[string]any{
			"file": filePath,
		}); err != nil {
			return err
		}
		c.stateMu.Lock()
		delete(c.openDocs, filePath)
		c.stateMu.Unlock()
	}
	if err := c.notify("open", map[string]any{
		"file":        filePath,
		"fileContent": content,
	}); err != nil {
		return err
	}
	c.stateMu.Lock()
	c.openDocs[filePath] = content
	c.stateMu.Unlock()
	return nil
}

func (c *repoHoverTSServerClient) readLoop() {
	reader := bufio.NewReader(c.stdout)
	for {
		message, err := readLSPMessage(reader)
		if err != nil {
			c.failPending(err)
			return
		}

		var base struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(message, &base); err != nil {
			continue
		}

		switch base.Type {
		case "response":
			var response tsServerResponseEnvelope
			if err := json.Unmarshal(message, &response); err != nil {
				continue
			}
			c.stateMu.Lock()
			ch := c.pending[response.RequestSeq]
			delete(c.pending, response.RequestSeq)
			c.stateMu.Unlock()
			if ch != nil {
				ch <- response
				close(ch)
			}
		case "event":
			var event tsServerEventEnvelope
			if err := json.Unmarshal(message, &event); err != nil {
				continue
			}
		}
	}
}

func (c *repoHoverTSServerClient) failPending(err error) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	for requestSeq, ch := range c.pending {
		ch <- tsServerResponseEnvelope{Success: false, Message: err.Error()}
		close(ch)
		delete(c.pending, requestSeq)
	}
	select {
	case <-c.done:
	default:
		close(c.done)
	}
}

func (c *repoHoverTSServerClient) request(ctx context.Context, command string, arguments any) (json.RawMessage, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	requestCtx, cancel := context.WithTimeout(ctx, repoHoverTimeout)
	defer cancel()

	requestSeq := c.nextID.Add(1)
	responseCh := make(chan tsServerResponseEnvelope, 1)

	c.stateMu.Lock()
	c.pending[requestSeq] = responseCh
	c.stateMu.Unlock()

	if err := c.writeRequest(tsServerRequestEnvelope{
		Seq:       requestSeq,
		Type:      "request",
		Command:   command,
		Arguments: arguments,
	}); err != nil {
		c.stateMu.Lock()
		delete(c.pending, requestSeq)
		c.stateMu.Unlock()
		return nil, err
	}

	select {
	case <-requestCtx.Done():
		c.stateMu.Lock()
		delete(c.pending, requestSeq)
		c.stateMu.Unlock()
		return nil, requestCtx.Err()
	case <-c.done:
		return nil, errors.New(strings.TrimSpace(c.stderrBuf.String()))
	case response := <-responseCh:
		if !response.Success && command != "open" && command != "close" {
			if response.Message == "" {
				response.Message = "tsserver request failed"
			}
			return nil, errors.New(response.Message)
		}
		return response.Body, nil
	}
}

func (c *repoHoverTSServerClient) notify(command string, arguments any) error {
	return c.writeRequest(tsServerRequestEnvelope{
		Seq:       c.nextID.Add(1),
		Type:      "request",
		Command:   command,
		Arguments: arguments,
	})
}

func (c *repoHoverTSServerClient) writeRequest(request tsServerRequestEnvelope) error {
	payload, err := json.Marshal(request)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, err = c.stdin.Write(append(payload, '\n'))
	return err
}

func renderTSServerTags(tags []tsServerTag) string {
	if len(tags) == 0 {
		return ""
	}
	lines := make([]string, 0, len(tags))
	for _, tag := range tags {
		text := strings.TrimSpace(renderTSServerTagText(tag.Text))
		if text == "" {
			lines = append(lines, "@"+tag.Name)
			continue
		}
		lines = append(lines, "@"+tag.Name+" "+text)
	}
	return strings.Join(lines, "\n")
}

func renderTSServerDocumentation(documentation any) string {
	switch value := documentation.(type) {
	case string:
		return value
	case []any:
		var builder strings.Builder
		for _, item := range value {
			if part, ok := item.(map[string]any); ok {
				if rawText, ok := part["text"].(string); ok {
					builder.WriteString(rawText)
				}
				continue
			}
			builder.WriteString(fmt.Sprint(item))
		}
		return builder.String()
	case []tsServerDisplayPart:
		var builder strings.Builder
		for _, part := range value {
			builder.WriteString(part.Text)
		}
		return builder.String()
	default:
		return fmt.Sprint(documentation)
	}
}

func renderTSServerTagText(text any) string {
	switch value := text.(type) {
	case string:
		return value
	case []any:
		var builder strings.Builder
		for _, item := range value {
			if part, ok := item.(map[string]any); ok {
				if rawText, ok := part["text"].(string); ok {
					builder.WriteString(rawText)
				}
				continue
			}
			builder.WriteString(fmt.Sprint(item))
		}
		return builder.String()
	default:
		return fmt.Sprint(text)
	}
}
