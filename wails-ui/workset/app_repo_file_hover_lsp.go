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
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const repoHoverTimeout = 5 * time.Second

type repoHoverLSPRequest struct {
	filePath   string
	path       string
	content    string
	line       int
	character  int
	languageID string
	language   string
	provider   string
}

type repoHoverLSPClient struct {
	runtime   repoHoverRuntime
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	stderrBuf bytes.Buffer

	writeMu  sync.Mutex
	stateMu  sync.Mutex
	pending  map[int64]chan lspResponseEnvelope
	openDocs map[string]string
	nextID   atomic.Int64
	done     chan struct{}
}

type lspEnvelope struct {
	JSONRPC string          `json:"jsonrpc,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *lspError       `json:"error,omitempty"`
}

type lspError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type lspResponseEnvelope struct {
	result json.RawMessage
	err    *lspError
}

type lspInitializeResult struct {
	Capabilities struct {
		HoverProvider any `json:"hoverProvider"`
	} `json:"capabilities"`
}

type lspHoverParams struct {
	TextDocument lspTextDocumentIdentifier `json:"textDocument"`
	Position     lspPosition               `json:"position"`
}

type lspHoverResult struct {
	Contents any       `json:"contents"`
	Range    *lspRange `json:"range,omitempty"`
}

type lspDefinitionResult struct {
	URI                  string    `json:"uri,omitempty"`
	Range                *lspRange `json:"range,omitempty"`
	TargetURI            string    `json:"targetUri,omitempty"`
	TargetRange          *lspRange `json:"targetRange,omitempty"`
	TargetSelectionRange *lspRange `json:"targetSelectionRange,omitempty"`
}

type lspTextDocumentIdentifier struct {
	URI string `json:"uri"`
}

type lspVersionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

type lspTextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

type lspDidOpenParams struct {
	TextDocument lspTextDocumentItem `json:"textDocument"`
}

type lspDidCloseParams struct {
	TextDocument lspTextDocumentIdentifier `json:"textDocument"`
}

type lspPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type lspRange struct {
	Start lspPosition `json:"start"`
	End   lspPosition `json:"end"`
}

func newLSPHoverBackend(ctx context.Context, runtime repoHoverRuntime) (repoHoverBackend, error) {
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

	client := &repoHoverLSPClient{
		runtime:  runtime,
		cmd:      cmd,
		stdin:    stdin,
		stdout:   stdout,
		pending:  make(map[int64]chan lspResponseEnvelope),
		openDocs: make(map[string]string),
		done:     make(chan struct{}),
	}
	go func() {
		_, _ = io.Copy(&client.stderrBuf, stderr)
	}()
	go client.readLoop()

	if err := client.initialize(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

func (c *repoHoverLSPClient) Hover(ctx context.Context, request repoHoverLSPRequest) (RepoFileHoverResponse, error) {
	if !c.Alive() {
		return RepoFileHoverResponse{}, errors.New("hover backend is not running")
	}

	uri := pathToFileURI(request.filePath)
	if err := c.syncDocument(uri, request.languageID, request.content); err != nil {
		return RepoFileHoverResponse{}, err
	}

	result, err := c.request(ctx, "textDocument/hover", lspHoverParams{
		TextDocument: lspTextDocumentIdentifier{URI: uri},
		Position: lspPosition{
			Line:      request.line,
			Character: request.character,
		},
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

	var hover lspHoverResult
	if err := json.Unmarshal(result, &hover); err != nil {
		return RepoFileHoverResponse{}, err
	}

	header, documentation, documentationKind := normalizeLSPHoverContents(hover.Contents)
	response := RepoFileHoverResponse{
		Supported:         true,
		Available:         true,
		Found:             header != "" || documentation != "",
		Language:          request.language,
		Provider:          request.provider,
		Header:            header,
		Documentation:     documentation,
		DocumentationKind: documentationKind,
		Source:            filepath.Base(request.path),
	}
	if hover.Range != nil {
		response.Range = &RepoFileHoverRange{
			StartLine:      hover.Range.Start.Line,
			StartCharacter: hover.Range.Start.Character,
			EndLine:        hover.Range.End.Line,
			EndCharacter:   hover.Range.End.Character,
		}
	}
	return response, nil
}

func (c *repoHoverLSPClient) Definition(ctx context.Context, request repoHoverLSPRequest) ([]repoFileDefinitionLocation, error) {
	if !c.Alive() {
		return nil, errors.New("hover backend is not running")
	}

	uri := pathToFileURI(request.filePath)
	if err := c.syncDocument(uri, request.languageID, request.content); err != nil {
		return nil, err
	}

	result, err := c.request(ctx, "textDocument/definition", lspHoverParams{
		TextDocument: lspTextDocumentIdentifier{URI: uri},
		Position: lspPosition{
			Line:      request.line,
			Character: request.character,
		},
	})
	if err != nil {
		return nil, err
	}
	if len(result) == 0 || bytes.Equal(result, []byte("null")) {
		return nil, nil
	}

	locations, err := normalizeLSPDefinitionResult(result)
	if err != nil {
		return nil, err
	}
	targets := make([]repoFileDefinitionLocation, 0, len(locations))
	for _, location := range locations {
		targetPath := fileURIToPath(firstNonEmpty(location.TargetURI, location.URI))
		if targetPath == "" {
			continue
		}
		targetRange := location.TargetSelectionRange
		if targetRange == nil {
			targetRange = location.TargetRange
		}
		if targetRange == nil {
			targetRange = location.Range
		}
		if targetRange == nil {
			continue
		}
		targets = append(targets, repoFileDefinitionLocation{
			filePath:       targetPath,
			startLine:      targetRange.Start.Line,
			startCharacter: targetRange.Start.Character,
			endLine:        targetRange.End.Line,
			endCharacter:   targetRange.End.Character,
		})
	}
	return targets, nil
}

func (c *repoHoverLSPClient) Close() error {
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

func (c *repoHoverLSPClient) Alive() bool {
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

func (c *repoHoverLSPClient) initialize() error {
	initResult, err := c.request(context.Background(), "initialize", map[string]any{
		"processId": nil,
		"rootUri":   pathToFileURI(c.runtime.rootPath),
		"capabilities": map[string]any{
			"textDocument": map[string]any{
				"hover": map[string]any{
					"contentFormat": []string{"markdown", "plaintext"},
				},
			},
			"workspace": map[string]any{
				"workspaceFolders": true,
				"configuration":    true,
			},
		},
		"workspaceFolders": []map[string]string{
			{
				"uri":  pathToFileURI(c.runtime.rootPath),
				"name": filepath.Base(c.runtime.rootPath),
			},
		},
		"clientInfo": map[string]string{
			"name":    "workset",
			"version": "0.0.0",
		},
	})
	if err != nil {
		return fmt.Errorf("initialize %s: %w", c.runtime.provider, err)
	}
	var initializeResult lspInitializeResult
	if err := json.Unmarshal(initResult, &initializeResult); err != nil {
		return fmt.Errorf("decode %s initialize result: %w", c.runtime.provider, err)
	}
	if err := c.notify("initialized", map[string]any{}); err != nil {
		return err
	}
	return nil
}

func (c *repoHoverLSPClient) syncDocument(uri, languageID, content string) error {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()

	if existing, ok := c.openDocs[uri]; ok && existing == content {
		return nil
	}
	if _, ok := c.openDocs[uri]; ok {
		if err := c.notify("textDocument/didClose", lspDidCloseParams{
			TextDocument: lspTextDocumentIdentifier{URI: uri},
		}); err != nil {
			return err
		}
		delete(c.openDocs, uri)
	}
	if err := c.notify("textDocument/didOpen", lspDidOpenParams{
		TextDocument: lspTextDocumentItem{
			URI:        uri,
			LanguageID: languageID,
			Version:    1,
			Text:       content,
		},
	}); err != nil {
		return err
	}
	c.openDocs[uri] = content
	return nil
}

func (c *repoHoverLSPClient) readLoop() {
	reader := bufio.NewReader(c.stdout)
	for {
		message, err := readLSPMessage(reader)
		if err != nil {
			c.failPending(err)
			return
		}
		var envelope lspEnvelope
		if err := json.Unmarshal(message, &envelope); err != nil {
			c.failPending(err)
			return
		}
		switch {
		case envelope.Method != "" && len(envelope.ID) > 0:
			c.handleServerRequest(envelope)
		case len(envelope.ID) > 0:
			id, ok := parseLSPID(envelope.ID)
			if !ok {
				continue
			}
			c.stateMu.Lock()
			ch := c.pending[id]
			if ch != nil {
				delete(c.pending, id)
			}
			c.stateMu.Unlock()
			if ch != nil {
				ch <- lspResponseEnvelope{result: envelope.Result, err: envelope.Error}
			}
		default:
			// Ignore notifications for now.
		}
	}
}

func (c *repoHoverLSPClient) handleServerRequest(envelope lspEnvelope) {
	var result any
	if envelope.Method == "workspace/configuration" {
		var params struct {
			Items []any `json:"items"`
		}
		_ = json.Unmarshal(envelope.Params, &params)
		result = make([]any, len(params.Items))
	}
	_ = c.writeEnvelope(map[string]any{
		"jsonrpc": "2.0",
		"id":      json.RawMessage(envelope.ID),
		"result":  result,
	})
}

func (c *repoHoverLSPClient) failPending(err error) {
	select {
	case <-c.done:
	default:
		close(c.done)
	}

	c.stateMu.Lock()
	pending := c.pending
	c.pending = map[int64]chan lspResponseEnvelope{}
	c.stateMu.Unlock()

	response := lspResponseEnvelope{
		err: &lspError{
			Message: err.Error(),
		},
	}
	for _, ch := range pending {
		ch <- response
	}
}

func (c *repoHoverLSPClient) request(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := c.nextID.Add(1)
	ch := make(chan lspResponseEnvelope, 1)

	c.stateMu.Lock()
	c.pending[id] = ch
	c.stateMu.Unlock()

	if err := c.writeEnvelope(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}); err != nil {
		c.stateMu.Lock()
		delete(c.pending, id)
		c.stateMu.Unlock()
		return nil, err
	}

	waitCtx, cancel := context.WithTimeout(ctx, repoHoverTimeout)
	defer cancel()

	select {
	case response := <-ch:
		if response.err != nil {
			return nil, fmt.Errorf("%s: %s", method, response.err.Message)
		}
		return response.result, nil
	case <-waitCtx.Done():
		c.stateMu.Lock()
		delete(c.pending, id)
		c.stateMu.Unlock()
		return nil, waitCtx.Err()
	}
}

func (c *repoHoverLSPClient) notify(method string, params any) error {
	return c.writeEnvelope(map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	})
}

func (c *repoHoverLSPClient) writeEnvelope(payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if _, err := fmt.Fprintf(c.stdin, "Content-Length: %d\r\n\r\n", len(data)); err != nil {
		return err
	}
	_, err = c.stdin.Write(data)
	return err
}

func readLSPMessage(reader *bufio.Reader) ([]byte, error) {
	contentLength := -1
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(parts[0]), "Content-Length") {
			contentLength, err = strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil {
				return nil, err
			}
		}
	}
	if contentLength < 0 {
		return nil, fmt.Errorf("missing content length")
	}
	payload := make([]byte, contentLength)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func parseLSPID(raw json.RawMessage) (int64, bool) {
	var integerID int64
	if err := json.Unmarshal(raw, &integerID); err == nil {
		return integerID, true
	}
	var stringID string
	if err := json.Unmarshal(raw, &stringID); err == nil {
		parsed, err := strconv.ParseInt(stringID, 10, 64)
		return parsed, err == nil
	}
	return 0, false
}

func pathToFileURI(path string) string {
	path = filepath.Clean(path)
	path = filepath.ToSlash(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return "file://" + path
}

func fileURIToPath(uri string) string {
	path := strings.TrimSpace(uri)
	if path == "" {
		return ""
	}
	path = strings.TrimPrefix(path, "file://")
	if path == "" {
		return ""
	}
	return filepath.Clean(filepath.FromSlash(path))
}

func normalizeLSPDefinitionResult(payload json.RawMessage) ([]lspDefinitionResult, error) {
	var single lspDefinitionResult
	if err := json.Unmarshal(payload, &single); err == nil && (single.URI != "" || single.TargetURI != "") {
		return []lspDefinitionResult{single}, nil
	}

	var many []lspDefinitionResult
	if err := json.Unmarshal(payload, &many); err == nil {
		return many, nil
	}

	return nil, fmt.Errorf("decode definition result")
}

func normalizeLSPHoverContents(contents any) (header, documentation, documentationKind string) {
	switch value := contents.(type) {
	case string:
		return "", value, "markdown"
	case map[string]any:
		return normalizeSingleHoverEntry(value)
	case []any:
		var docs []string
		var kind string
		for _, item := range value {
			itemHeader, itemDoc, itemKind := normalizeSingleHoverEntry(item)
			if header == "" && itemHeader != "" {
				header = itemHeader
			}
			if itemDoc != "" {
				docs = append(docs, itemDoc)
				if kind == "" {
					kind = itemKind
				}
			}
		}
		return header, strings.Join(docs, "\n\n"), firstNonEmpty(kind, "markdown")
	default:
		return "", "", ""
	}
}

func normalizeSingleHoverEntry(value any) (header, documentation, documentationKind string) {
	switch entry := value.(type) {
	case string:
		return "", entry, "markdown"
	case map[string]any:
		if kind, ok := entry["kind"].(string); ok {
			text, _ := entry["value"].(string)
			return "", text, firstNonEmpty(kind, "markdown")
		}
		language, hasLanguage := entry["language"].(string)
		text, _ := entry["value"].(string)
		if hasLanguage && text != "" {
			return text, "", language
		}
		if text != "" {
			return "", text, "plaintext"
		}
	}
	return "", "", ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
