package manhwaindo

type readerPayload struct {
	PrevURL string         `json:"prevUrl"`
	NextURL string         `json:"nextUrl"`
	Sources []readerSource `json:"sources"`
}

type readerSource struct {
	Source string   `json:"source"`
	Images []string `json:"images"`
}
