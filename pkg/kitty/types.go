package kitty

type Cursor struct {
	Row int
	Col int
}

type Event struct {
	Kind      string     `json:"kind"`
	Image     *Image     `json:"image,omitempty"`
	Placement *Placement `json:"placement,omitempty"`
	Delete    *Delete    `json:"delete,omitempty"`
	Snapshot  *Snapshot  `json:"snapshot,omitempty"`
}

type Image struct {
	ID     string `json:"id"`
	Number uint32 `json:"number,omitempty"`
	Format string `json:"format"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
	Data   []byte `json:"data,omitempty"`
}

type Placement struct {
	ID      uint32 `json:"id"`
	ImageID string `json:"imageId"`
	Row     int    `json:"row"`
	Col     int    `json:"col"`
	Rows    int    `json:"rows"`
	Cols    int    `json:"cols"`
	X       int    `json:"x,omitempty"`
	Y       int    `json:"y,omitempty"`
	Z       int    `json:"z,omitempty"`
}

type Delete struct {
	All         bool   `json:"all,omitempty"`
	ImageID     string `json:"imageId,omitempty"`
	PlacementID uint32 `json:"placementId,omitempty"`
}

type Snapshot struct {
	Images     []Image     `json:"images"`
	Placements []Placement `json:"placements"`
}

type CursorMove struct {
	Cols int
	Rows int
}
