package images

import _ "embed"

var (
	//go:embed error.svg
	Error string
	//go:embed note.svg
	Note string
	//go:embed search.svg
	Search string
	//go:embed trash.svg
	Trash string
	//go:embed x-circle.svg
	XCircle string
	//go:embed keeper-logo.png
	Logo []byte
)
