package kitty

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

type State struct {
	mu            sync.Mutex
	images        map[string]*Image
	placements    map[string]*Placement
	nextPlacement uint32
	nextImage     uint32
}

func NewState() *State {
	return &State{
		images:     make(map[string]*Image),
		placements: make(map[string]*Placement),
	}
}

func (s *State) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	images := make([]Image, 0, len(s.images))
	for _, img := range s.images {
		if img == nil {
			continue
		}
		clone := *img
		images = append(images, clone)
	}
	placements := make([]Placement, 0, len(s.placements))
	for _, pl := range s.placements {
		if pl == nil {
			continue
		}
		clone := *pl
		placements = append(placements, clone)
	}
	return Snapshot{Images: images, Placements: placements}
}

func (s *State) Restore(snapshot Snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.images = make(map[string]*Image)
	s.placements = make(map[string]*Placement)
	var maxPlacement uint32
	var maxImage uint32
	for _, img := range snapshot.Images {
		clone := img
		s.images[img.ID] = &clone
		if id := parseAutoID(img.ID); id > maxImage {
			maxImage = id
		}
	}
	for _, pl := range snapshot.Placements {
		clone := pl
		key := placementKey(pl.ImageID, pl.ID)
		s.placements[key] = &clone
		if pl.ID > maxPlacement {
			maxPlacement = pl.ID
		}
	}
	s.nextPlacement = maxPlacement + 1
	s.nextImage = maxImage + 1
}

func (s *State) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.Snapshot())
}

func (s *State) UnmarshalJSON(data []byte) error {
	var snapshot Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return err
	}
	s.Restore(snapshot)
	return nil
}

func (s *State) ClearAll() []Event {
	s.mu.Lock()
	s.images = make(map[string]*Image)
	s.placements = make(map[string]*Placement)
	s.nextPlacement = 0
	s.mu.Unlock()
	return []Event{{Kind: "delete", Delete: &Delete{All: true}}}
}

func (s *State) Apply(cmd Command, cursor Cursor) (events []Event, move CursorMove) {
	s.mu.Lock()
	defer s.mu.Unlock()
	action := cmd.Action
	if action == "" {
		action = "t"
	}
	display := action == "T"
	switch strings.ToLower(action) {
	case "t":
		imageID := resolveImageID(cmd)
		if imageID == "" {
			s.nextImage++
			imageID = fmt.Sprintf("auto:%d", s.nextImage)
		}
		if len(cmd.Payload) == 0 {
			return nil, CursorMove{}
		}
		if _, ok := s.images[imageID]; ok {
			events = append(events, s.deleteImageLocked(imageID)...)
		}
		img := &Image{
			ID:     imageID,
			Number: cmd.Number,
			Format: cmd.Format,
			Width:  cmd.Width,
			Height: cmd.Height,
			Data:   cmd.Payload,
		}
		s.images[imageID] = img
		events = append(events, Event{Kind: "image", Image: img})
		if display {
			pl, cursorMove := s.placeLocked(imageID, cmd, cursor)
			if pl != nil {
				events = append(events, Event{Kind: "placement", Placement: pl})
				move = cursorMove
			}
		}
	case "p":
		imageID := resolveImageID(cmd)
		if imageID == "" {
			return nil, CursorMove{}
		}
		if _, ok := s.images[imageID]; !ok {
			return nil, CursorMove{}
		}
		pl, cursorMove := s.placeLocked(imageID, cmd, cursor)
		if pl != nil {
			events = append(events, Event{Kind: "placement", Placement: pl})
			move = cursorMove
		}
	case "d":
		events = append(events, s.deleteLocked(cmd, cursor)...)
	}
	return events, move
}

func (s *State) deleteLocked(cmd Command, cursor Cursor) []Event {
	switch cmd.DeleteMode {
	case "a", "A":
		s.images = make(map[string]*Image)
		s.placements = make(map[string]*Placement)
		s.nextPlacement = 0
		return []Event{{Kind: "delete", Delete: &Delete{All: true}}}
	case "i", "I":
		imageID := resolveImageID(cmd)
		if imageID == "" {
			return nil
		}
		return s.deleteImageLocked(imageID)
	case "p", "P":
		imageID := resolveImageID(cmd)
		if imageID == "" || cmd.PlacementID == 0 {
			return nil
		}
		key := placementKey(imageID, cmd.PlacementID)
		if _, ok := s.placements[key]; !ok {
			return nil
		}
		delete(s.placements, key)
		return []Event{{Kind: "delete", Delete: &Delete{PlacementID: cmd.PlacementID, ImageID: imageID}}}
	case "c", "C":
		return s.deleteAtCursorLocked(cursor)
	}
	return nil
}

func (s *State) deleteImageLocked(imageID string) []Event {
	delete(s.images, imageID)
	removed := []uint32{}
	for key, pl := range s.placements {
		if pl != nil && pl.ImageID == imageID {
			removed = append(removed, pl.ID)
			delete(s.placements, key)
		}
	}
	events := []Event{{Kind: "delete", Delete: &Delete{ImageID: imageID}}}
	for _, id := range removed {
		events = append(events, Event{Kind: "delete", Delete: &Delete{PlacementID: id, ImageID: imageID}})
	}
	return events
}

func (s *State) deleteAtCursorLocked(cursor Cursor) []Event {
	var events []Event
	for key, pl := range s.placements {
		if pl == nil {
			continue
		}
		if cursor.Row >= pl.Row && cursor.Row < pl.Row+max(1, pl.Rows) && cursor.Col >= pl.Col && cursor.Col < pl.Col+max(1, pl.Cols) {
			delete(s.placements, key)
			events = append(events, Event{Kind: "delete", Delete: &Delete{PlacementID: pl.ID, ImageID: pl.ImageID}})
		}
	}
	return events
}

func (s *State) placeLocked(imageID string, cmd Command, cursor Cursor) (*Placement, CursorMove) {
	cols := cmd.Cols
	rows := cmd.Rows
	if cols < 0 {
		cols = 0
	}
	if rows < 0 {
		rows = 0
	}
	placementID := cmd.PlacementID
	if placementID == 0 {
		s.nextPlacement++
		placementID = s.nextPlacement
	}
	pl := &Placement{
		ID:      placementID,
		ImageID: imageID,
		Row:     cursor.Row,
		Col:     cursor.Col,
		Rows:    rows,
		Cols:    cols,
		X:       cmd.X,
		Y:       cmd.Y,
		Z:       cmd.Z,
	}
	key := placementKey(imageID, placementID)
	s.placements[key] = pl
	if cmd.NoCursorMove {
		return pl, CursorMove{}
	}
	return pl, CursorMove{Cols: cols, Rows: rows}
}

func placementKey(imageID string, placementID uint32) string {
	if imageID == "" {
		return ""
	}
	return imageID + ":" + itoaU32(placementID)
}

func itoaU32(v uint32) string {
	if v == 0 {
		return "0"
	}
	buf := [10]byte{}
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func parseAutoID(id string) uint32 {
	if !strings.HasPrefix(id, "auto:") {
		return 0
	}
	value := strings.TrimPrefix(id, "auto:")
	parsed, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return 0
	}
	return uint32(parsed)
}
