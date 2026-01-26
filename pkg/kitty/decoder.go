package kitty

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const (
	esc = 0x1b
	bel = 0x07
)

type Command struct {
	Action       string
	Params       map[string]string
	Payload      []byte
	RawPayload   string
	More         bool
	Format       string
	Width        int
	Height       int
	Cols         int
	Rows         int
	X            int
	Y            int
	Z            int
	NoCursorMove bool
	ImageID      string
	Number       uint32
	PlacementID  uint32
	DeleteMode   string
	Compression  string
	Transmission string
}

type transfer struct {
	params  map[string]string
	action  string
	payload strings.Builder
}

type Decoder struct {
	pendingEsc      bool
	inAPC           bool
	apcEscPending   bool
	apcBuf          []byte
	pendingTransfer *transfer
}

func (d *Decoder) Process(data []byte, cursor Cursor, state *State) ([]byte, []Event) {
	if len(data) == 0 {
		return nil, nil
	}
	out := make([]byte, 0, len(data))
	var events []Event
	cur := cursor
	appendMove := func(move CursorMove) {
		if move.Cols > 0 {
			out = append(out, []byte(fmt.Sprintf("\x1b[%dC", move.Cols))...)
			cur.Col += move.Cols
		}
		if move.Rows > 0 {
			out = append(out, []byte(fmt.Sprintf("\x1b[%dB", move.Rows))...)
			cur.Row += move.Rows
		}
	}

	finalize := func(term []byte) {
		apc := append([]byte{}, d.apcBuf...)
		cmd, ok := parseKittyAPC(apc)
		d.apcBuf = d.apcBuf[:0]
		d.inAPC = false
		d.apcEscPending = false
		if !ok {
			out = append(out, esc)
			out = append(out, '_')
			out = append(out, apc...)
			out = append(out, term...)
			return
		}
		cmds := d.expandCommand(cmd)
		for _, resolved := range cmds {
			if state == nil {
				continue
			}
			ev, move := state.Apply(resolved, cur)
			events = append(events, ev...)
			appendMove(move)
		}
	}

	i := 0
	if d.pendingEsc {
		d.pendingEsc = false
		if len(data) == 0 {
			return out, events
		}
		if data[0] == '_' {
			d.inAPC = true
			i = 1
		} else {
			out = append(out, esc, data[0])
			i = 1
		}
	}

	for i < len(data) {
		if d.inAPC {
			if d.apcEscPending {
				if data[i] == '\\' {
					finalize([]byte{esc, '\\'})
					i++
					continue
				}
				d.apcBuf = append(d.apcBuf, esc, data[i])
				d.apcEscPending = false
				i++
				continue
			}
			switch data[i] {
			case bel:
				finalize([]byte{bel})
				i++
			case esc:
				d.apcEscPending = true
				i++
			default:
				d.apcBuf = append(d.apcBuf, data[i])
				i++
			}
			continue
		}
		if data[i] == esc {
			if i+1 >= len(data) {
				d.pendingEsc = true
				i++
				continue
			}
			if data[i+1] == '_' {
				d.inAPC = true
				d.apcBuf = d.apcBuf[:0]
				i += 2
				continue
			}
			out = append(out, data[i])
			i++
			continue
		}
		out = append(out, data[i])
		i++
	}
	return out, events
}

func (d *Decoder) expandCommand(cmd Command) []Command {
	if cmd.More {
		if d.pendingTransfer == nil {
			params := cmd.Params
			if params == nil {
				params = make(map[string]string)
			}
			d.pendingTransfer = &transfer{params: params, action: cmd.Action}
		} else {
			d.pendingTransfer.params = mergeParams(d.pendingTransfer.params, cmd.Params)
			if cmd.Action != "" {
				d.pendingTransfer.action = cmd.Action
			}
		}
		d.pendingTransfer.payload.WriteString(cmd.RawPayload)
		return nil
	}
	if d.pendingTransfer != nil {
		d.pendingTransfer.params = mergeParams(d.pendingTransfer.params, cmd.Params)
		if cmd.Action == "" {
			cmd.Action = d.pendingTransfer.action
		}
		cmd.Params = d.pendingTransfer.params
		d.pendingTransfer.payload.WriteString(cmd.RawPayload)
		cmd.RawPayload = d.pendingTransfer.payload.String()
		d.pendingTransfer = nil
	}
	resolved, ok := resolveCommand(cmd)
	if !ok {
		return nil
	}
	return []Command{resolved}
}

func parseKittyAPC(apc []byte) (Command, bool) {
	if len(apc) == 0 || apc[0] != 'G' {
		return Command{}, false
	}
	payload := ""
	control := string(apc[1:])
	if idx := strings.Index(control, ";"); idx >= 0 {
		payload = control[idx+1:]
		control = control[:idx]
	}
	params := parseParams(control)
	action := params["a"]
	more := params["m"] == "1"
	return Command{Action: action, Params: params, RawPayload: payload, More: more}, true
}

func parseParams(control string) map[string]string {
	params := make(map[string]string)
	if control == "" {
		return params
	}
	parts := strings.Split(control, ",")
	for _, part := range parts {
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		params[kv[0]] = kv[1]
	}
	return params
}

func mergeParams(dst, src map[string]string) map[string]string {
	if src == nil {
		return dst
	}
	if dst == nil {
		dst = make(map[string]string, len(src))
	}
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func resolveCommand(cmd Command) (Command, bool) {
	cmd.Format = parseFormat(cmd.Params)
	cmd.Width = parseInt(cmd.Params["s"])
	cmd.Height = parseInt(cmd.Params["v"])
	cmd.Cols = parseInt(cmd.Params["c"])
	cmd.Rows = parseInt(cmd.Params["r"])
	cmd.X = parseInt(cmd.Params["x"])
	cmd.Y = parseInt(cmd.Params["y"])
	cmd.Z = parseInt(cmd.Params["z"])
	cmd.NoCursorMove = cmd.Params["C"] == "1"
	cmd.ImageID = cmd.Params["i"]
	cmd.Number = parseUint(cmd.Params["I"])
	cmd.PlacementID = parseUint(cmd.Params["p"])
	cmd.DeleteMode = cmd.Params["d"]
	cmd.Compression = cmd.Params["o"]
	cmd.Transmission = cmd.Params["t"]

	if cmd.Action == "" {
		cmd.Action = "t"
	}
	if cmd.Action == "d" || cmd.Action == "p" {
		return cmd, true
	}
	if cmd.Transmission != "" && cmd.Transmission != "d" {
		return Command{}, false
	}
	payload, ok := decodePayload(cmd.RawPayload, cmd.Compression)
	if !ok {
		return Command{}, false
	}
	cmd.Payload = payload
	return cmd, true
}

func decodePayload(raw string, compression string) ([]byte, bool) {
	clean := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' || r == ' ' {
			return -1
		}
		return r
	}, raw)
	if clean == "" {
		return nil, false
	}
	decoded, err := base64.StdEncoding.DecodeString(clean)
	if err != nil {
		return nil, false
	}
	if compression == "z" {
		zr, err := zlib.NewReader(bytes.NewReader(decoded))
		if err != nil {
			return nil, false
		}
		defer func() {
			_ = zr.Close()
		}()
		out, err := io.ReadAll(zr)
		if err != nil {
			return nil, false
		}
		return out, true
	}
	return decoded, true
}

func parseFormat(params map[string]string) string {
	format := params["f"]
	switch format {
	case "24":
		return "rgb"
	case "32":
		return "rgba"
	case "100":
		return "png"
	default:
		if format != "" {
			return format
		}
		return "png"
	}
}

func parseInt(v string) int {
	if v == "" {
		return 0
	}
	parsed, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return parsed
}

func parseUint(v string) uint32 {
	if v == "" {
		return 0
	}
	parsed, err := strconv.ParseUint(v, 10, 32)
	if err != nil {
		return 0
	}
	return uint32(parsed)
}

func resolveImageID(cmd Command) string {
	if cmd.ImageID != "" {
		return cmd.ImageID
	}
	if cmd.Number != 0 {
		return fmt.Sprintf("I:%d", cmd.Number)
	}
	return ""
}
