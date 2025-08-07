package ui

import (
	"Keeper/app/utils/svgs"
	"Keeper/res/images"
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"os"
	"slices"
	"strings"

	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

var (
	th                 *material.Theme
	searchBar          widget.Editor
	titleEntry         widget.Editor
	noteEditor         widget.Editor
	notesList          widget.List
	addNoteBtn         widget.Clickable
	msg                msgBox
	prompt             confirmPrompt
	notes              []note
	scratchNotes       []note
	addNoteBtnDisabled bool
	editorOpen         bool
	currentInd         int
	// icons
	logo, noteIco, searchIco, trashIco, errorIco, xcircle image.Image
)

type note struct {
	title, content string
	isSelected     bool
}

type (
	C = layout.Context
	D = layout.Dimensions
)

const (
	NOTES_SAVE_PATH = "data/notes.bin"
	NOTES_SAVE_DIR  = "data"
	XOR_KEY         = "k@@P*Robfuscated"
)

const (
	IMG_PATH       = "res/images/"
	MSG_BOX_HEIGHT = 25
)

func init() {
	th = material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	th.Fg = color.NRGBA{250, 249, 246, 255}
	searchBar = widget.Editor{
		SingleLine: true,
		Submit:     true,
	}
	titleEntry = widget.Editor{
		SingleLine: true,
		Submit:     true,
	}
	noteEditor = widget.Editor{
		SingleLine: false,
		Submit:     false,
	}

	notesList = widget.List{}
	notesList.Axis = layout.Vertical
	// Init message box
	msg = msgBox{height: MSG_BOX_HEIGHT, offsetY: MSG_BOX_HEIGHT} // offset it so that it is hidden initially
	// Init popup
	prompt = confirmPrompt{w: 350, h: 140}
	// Loading icons to memory as raw pixel format
	logo, _ = png.Decode(bytes.NewReader(images.Logo))
	noteIco, _ = svgs.LoadSvg(strings.NewReader(images.Note), image.Point{})
	searchIco, _ = svgs.LoadSvg(strings.NewReader(images.Search), image.Point{})
	trashIco, _ = svgs.LoadSvg(strings.NewReader(images.Trash), image.Point{})
	errorIco, _ = svgs.LoadSvg(strings.NewReader(images.Error), image.Point{48, 48})
	xcircle, _ = svgs.LoadSvg(strings.NewReader(images.XCircle), image.Pt(18, 18))
}

func searchNUpdateNotes(title string) {
	title = strings.ToLower(title)
	currentInd = -1
	if len(notes) == 0 && len(scratchNotes) == 0 {
		return
	}
	var tempNotes []note
	if scratchNotes == nil {
		for _, v := range notes {
			if strings.Contains(strings.ToLower(v.title), title) {
				tempNotes = append(tempNotes, v)
			}
		}
		scratchNotes = notes
	} else {
		for _, v := range scratchNotes {
			if strings.Contains(strings.ToLower(v.title), title) {
				tempNotes = append(tempNotes, v)
			}
		}
	}
	notes = tempNotes
}

func disabledAddNoteBtn(btn material.ButtonStyle, gtx C) D {
	macro := op.Record(gtx.Ops)
	dims := btn.Layout(gtx)
	call := macro.Stop()
	call.Add(gtx.Ops)
	defer clip.UniformRRect(image.Rect(0, 0, dims.Size.X, dims.Size.Y), 3).Push(gtx.Ops).Pop()
	paint.ColorOp{Color: color.NRGBA{B: 200, A: 190}}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	event.Op(gtx.Ops, &addNoteBtnDisabled)
	return dims
}

func notesListItem(gtx C, index int) D {
	macro := op.Record(gtx.Ops)
	f := func(gtx C) D {
		return layout.Flex{
			Axis:      layout.Horizontal,
			Alignment: layout.Middle,
		}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return widget.Image{Src: paint.NewImageOp(noteIco)}.Layout(gtx)
			}),
			layout.Flexed(0.5, func(gtx C) D {
				return material.Label(th, unit.Sp(13), notes[index].title).Layout(gtx)
			}),
		)
	}
	var dims layout.Dimensions
	if notes[index].isSelected {
		// Selected note item (background included)
		dims = layout.Background{}.Layout(gtx,
			func(gtx C) D {
				x, y := gtx.Constraints.Min.X, gtx.Constraints.Min.Y
				defer clip.UniformRRect(image.Rect(0, 0, x, y), 10).Push(gtx.Ops).Pop()
				paint.ColorOp{Color: color.NRGBA{B: 155, A: 155}}.Add(gtx.Ops)
				paint.PaintOp{}.Add(gtx.Ops)
				return D{Size: image.Pt(x, y)}
			}, f,
		)
	} else {
		// Unselected note item (background not included)
		dims = f(gtx)
	}
	stack := clip.Rect{Max: dims.Size}.Push(gtx.Ops)
	tag := &notes[index]
	event.Op(gtx.Ops, tag)
	// Check for events
	for {
		ev, ok := gtx.Source.Event(pointer.Filter{
			Target: tag,
			Kinds:  pointer.Press | pointer.Release,
		})
		if !ok {
			break
		}
		if x, ok := ev.(pointer.Event); ok {
			switch x.Kind {
			case pointer.Press:
				for i := range notes {
					if notes[i].isSelected {
						notes[i].isSelected = !notes[i].isSelected
					}
				}
				editorOpen = true
				currentInd = index
				notes[currentInd].isSelected = true
				titleEntry.SetText(notes[currentInd].title)
				noteEditor.SetText(notes[currentInd].content)
				gtx.Execute(op.InvalidateCmd{})
			}
		}
	}
	stack.Pop()
	call := macro.Stop()
	// Extra space given to top (ignored for index 0)
	offset := 0
	if index != 0 {
		offset = 7
	}
	defer op.Offset(image.Pt(0, offset)).Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: image.Pt(dims.Size.X, dims.Size.Y+offset)}
}

func notesPane(gtx C) D {
	maxW := 230
	return layout.Flex{
		Axis: layout.Vertical,
	}.Layout(gtx,
		// Search bar
		layout.Rigid(func(gtx C) D {
			// Get the last search
			prevSearch := searchBar.Text()
			// Layout everything
			gtx.Constraints.Max.X, gtx.Constraints.Min.X = maxW, maxW
			dims := layout.Background{}.Layout(gtx,
				// Set a background
				func(gtx C) D {
					sz := gtx.Constraints.Min
					defer clip.UniformRRect(image.Rect(0, 0, sz.X, sz.Y), 5).Push(gtx.Ops).Pop()
					paint.ColorOp{Color: color.NRGBA{255, 255, 255, 20}}.Add(gtx.Ops)
					paint.PaintOp{}.Add(gtx.Ops)
					return layout.Dimensions{Size: sz}
				},
				// Layout the search box
				func(gtx C) D {
					return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx C) D {
						edit := material.Editor(th, &searchBar, "Search")
						edit.Font.Style = font.Italic
						edit.TextSize = unit.Sp(14)
						img := widget.Image{Src: paint.NewImageOp(searchIco)}
						return layout.Flex{
							Axis:      layout.Horizontal,
							Alignment: layout.Middle,
						}.Layout(gtx,
							layout.Rigid(img.Layout),
							layout.Rigid(layout.Spacer{Width: unit.Dp(3)}.Layout),
							layout.Flexed(0.5, edit.Layout),
						)
					})
				},
			)
			// Update states
			if s := searchBar.Text(); s != prevSearch {
				if s == "" {
					addNoteBtnDisabled = false
					if scratchNotes != nil {
						notes = scratchNotes
						scratchNotes = nil
					}
				} else {
					addNoteBtnDisabled = true
					editorOpen = false
					if len(notes) != 0 && currentInd != -1 {
						notes[currentInd].isSelected = false
					}
					searchNUpdateNotes(s)
				}
				gtx.Execute(op.InvalidateCmd{})
			}
			return dims
		}),
		// Spacer
		layout.Rigid(layout.Spacer{Height: unit.Dp(7)}.Layout),
		// List of notes
		layout.Flexed(0.5, func(gtx C) D {
			gtx.Constraints.Max.X, gtx.Constraints.Min.X = maxW, maxW
			return layout.Background{}.Layout(gtx,
				// Set a background
				func(gtx C) D {
					sz := gtx.Constraints.Min
					defer clip.UniformRRect(image.Rect(0, 0, sz.X, sz.Y), 7).Push(gtx.Ops).Pop()
					paint.ColorOp{Color: color.NRGBA{27, 27, 30, 255}}.Add(gtx.Ops)
					paint.PaintOp{}.Add(gtx.Ops)
					return layout.Dimensions{Size: sz}
				},
				// Layout the list
				func(gtx C) D {
					return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx C) D {
						return material.List(th, &notesList).Layout(gtx, len(notes), notesListItem)
					})
				},
			)
		}),
		// Spacer
		layout.Rigid(layout.Spacer{Height: unit.Dp(7)}.Layout),
		// 'Add Note' button
		layout.Rigid(func(gtx C) D {
			if addNoteBtn.Clicked(gtx) {
				notes = append(notes, note{title: "Untitled"})
			}
			gtx.Constraints.Max.X, gtx.Constraints.Min.X = maxW, maxW
			var dims layout.Dimensions
			btn := material.Button(th, &addNoteBtn, "Add Note")
			if addNoteBtnDisabled {
				dims = disabledAddNoteBtn(btn, gtx)
			} else {
				dims = btn.Layout(gtx)
			}
			return dims
		}),
	)
}

func editorPane(gtx C) D {
	return layout.Flex{
		Axis: layout.Vertical,
	}.Layout(gtx,
		// Top row
		layout.Rigid(func(gtx C) D {
			return layout.Flex{
				Axis:      layout.Horizontal,
				Alignment: layout.Middle,
			}.Layout(gtx,
				// Title entry
				layout.Flexed(0.5, func(gtx C) D {
					// Get the last title
					prevTitle := titleEntry.Text()
					// Layout everything
					edit := material.Editor(th, &titleEntry, "Title")
					edit.Font.Style = font.Italic
					edit.TextSize = unit.Sp(16)
					dims := layout.Background{}.Layout(gtx,
						// Set a background
						func(gtx C) D {
							sz := gtx.Constraints.Min
							defer clip.UniformRRect(image.Rect(0, 0, sz.X, sz.Y), 5).Push(gtx.Ops).Pop()
							paint.ColorOp{Color: color.NRGBA{255, 255, 255, 20}}.Add(gtx.Ops)
							paint.PaintOp{}.Add(gtx.Ops)
							return layout.Dimensions{Size: sz}
						},
						// Layout the entry
						func(gtx C) D {
							return layout.UniformInset(unit.Dp(7)).Layout(gtx, edit.Layout)
						},
					)
					// Update states
					if s := titleEntry.Text(); s != prevTitle {
						notes[currentInd].title = s
						gtx.Execute(op.InvalidateCmd{})
					}
					return dims
				}),
				// Spacer
				layout.Rigid(layout.Spacer{Width: unit.Dp(5)}.Layout),
				// Trash button
				layout.Rigid(func(gtx C) D {
					macro := op.Record(gtx.Ops)
					dims := widget.Image{Src: paint.NewImageOp(trashIco)}.Layout(gtx)
					call := macro.Stop()
					// Register and check for events
					defer clip.Rect{Max: dims.Size}.Push(gtx.Ops).Pop()
					event.Op(gtx.Ops, &trashIco)
					for {
						ev, ok := gtx.Source.Event(pointer.Filter{
							Target: &trashIco,
							Kinds:  pointer.Press | pointer.Release,
						})
						if !ok {
							break
						}
						if x, ok := ev.(pointer.Event); ok {
							switch x.Kind {
							case pointer.Release:
								prompt.onConfirm = func() {
									// Delete note pointed to by currentInd
									t := notes[currentInd].title
									c := notes[currentInd].content
									notes = slices.Delete(notes, currentInd, currentInd+1)
									for i := range scratchNotes {
										b := strings.Contains(scratchNotes[i].title, t) &&
											strings.Contains(scratchNotes[i].content, c)
										if b {
											scratchNotes = slices.Delete(scratchNotes, i, i+1)
											break
										}
									}
									editorOpen = !editorOpen
									currentInd = -1
									msg.resetOffset()
									msg.isAnimating = true
								}
								prompt.isPromptOpen = !prompt.isPromptOpen
								gtx.Execute(op.InvalidateCmd{})
							}
						}
					}
					// Layout the widget
					call.Add(gtx.Ops)
					return dims
				}),
			)
		}),
		// Spacer
		layout.Rigid(layout.Spacer{Height: unit.Dp(7)}.Layout),
		// Note editor
		layout.Flexed(0.5, func(gtx C) D {
			// Get the last note text
			prevNote := noteEditor.Text()
			// Layout everything
			edit := material.Editor(th, &noteEditor, "Write something...")
			edit.TextSize = unit.Sp(14)
			dims := layout.Background{}.Layout(gtx,
				// Set a background
				func(gtx C) D {
					sz := gtx.Constraints.Min
					defer clip.UniformRRect(image.Rect(0, 0, sz.X, sz.Y), 5).Push(gtx.Ops).Pop()
					paint.ColorOp{Color: color.NRGBA{23, 23, 26, 255}}.Add(gtx.Ops)
					paint.PaintOp{}.Add(gtx.Ops)
					return layout.Dimensions{Size: sz}
				},
				// Layout the note editor
				func(gtx C) D {
					return layout.UniformInset(unit.Dp(8)).Layout(gtx, edit.Layout)
				},
			)
			// Update states
			if s := noteEditor.Text(); s != prevNote {
				notes[currentInd].content = s
				gtx.Execute(op.InvalidateCmd{})
			}
			return dims
		}),
	)
}

type msgBox struct {
	height      int
	isAnimating bool
	offsetY     float32
}

func (msgBox) layout(gtx C) D {
	gtx.Constraints.Max.Y, gtx.Constraints.Min.Y = msg.height, msg.height
	macro := op.Record(gtx.Ops)
	dims := layout.Background{}.Layout(gtx,
		func(gtx C) D {
			sz := gtx.Constraints.Min
			defer clip.UniformRRect(image.Rect(0, 0, sz.X, sz.Y), 5).Push(gtx.Ops).Pop()
			paint.ColorOp{Color: color.NRGBA{70, 219, 88, 255}}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			return D{Size: sz}
		},
		func(gtx C) D {
			return layout.Flex{
				Axis:      layout.Horizontal,
				Alignment: layout.Middle,
			}.Layout(gtx,
				// Message
				layout.Flexed(0.5, func(gtx C) D {
					cgtx := gtx
					cgtx.Constraints.Min.Y = 14
					th_ := *th
					th_.Fg = color.NRGBA{23, 27, 23, 255}
					lbl := material.Label(&th_, unit.Sp(14), "Successfully deleted note!")
					lbl.Font.Weight = font.ExtraBold
					lbl.Alignment = text.Middle
					return lbl.Layout(cgtx)
				}),
				// Cross button to hide msgbox from view
				layout.Rigid(func(gtx C) D {
					img := widget.Image{Src: paint.NewImageOp(xcircle)}
					img.Position = layout.Center
					macro := op.Record(gtx.Ops)
					dims := img.Layout(gtx)
					call := macro.Stop()
					// Tag event area
					defer clip.Rect{Max: dims.Size}.Push(gtx.Ops).Pop()
					event.Op(gtx.Ops, &xcircle)
					// Check for events
					for {
						ev, ok := gtx.Source.Event(pointer.Filter{
							Target: &xcircle,
							Kinds:  pointer.Press | pointer.Release,
						})
						if !ok {
							break
						}
						if x, ok := ev.(pointer.Event); ok {
							switch x.Kind {
							case pointer.Release:
								msg.resetOffset()
								gtx.Execute(op.InvalidateCmd{})
							}
						}
					}
					call.Add(gtx.Ops)
					return dims
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(3)}.Layout),
			)
		},
	)
	call := macro.Stop()
	if msg.isAnimating {
		msg.offsetY -= 0.7
		if msg.offsetY < 0 {
			msg.offsetY = 0
			msg.isAnimating = false
		}
		gtx.Execute(op.InvalidateCmd{})
	}
	defer clip.Rect{Max: dims.Size}.Push(gtx.Ops).Pop()
	defer op.Offset(image.Pt(0, int(msg.offsetY))).Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
	return dims
}

func (msgBox) resetOffset() {
	msg.offsetY = MSG_BOX_HEIGHT
}

type confirmPrompt struct {
	onConfirm                   func()
	promptCancel, promptConfirm widget.Clickable
	isPromptOpen                bool
	backdropT                   int
	w, h                        int
}

func (confirmPrompt) layout(gtx C) D {
	// Set a backdrop
	trans := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
	paint.ColorOp{Color: color.NRGBA{A: 180}}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	event.Op(gtx.Ops, &prompt.backdropT)
	trans.Pop()
	// Save constraints and resize constraints min max
	pt := image.Pt(prompt.w, prompt.h)
	savedConstraints := gtx.Constraints
	gtx.Constraints.Min, gtx.Constraints.Max = pt, pt
	macro := op.Record(gtx.Ops)
	dims := layout.Background{}.Layout(gtx,
		// Set a background to popup
		func(gtx C) D {
			min := gtx.Constraints.Min
			defer clip.UniformRRect(image.Rect(0, 0, min.X, min.Y), 5).Push(gtx.Ops).Pop()
			paint.ColorOp{Color: color.NRGBA{60, 60, 63, 255}}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			return layout.Dimensions{Size: min}
		},
		// Layout popup
		func(gtx C) D {
			// Process popup actions
			if prompt.promptCancel.Clicked(gtx) {
				prompt.isPromptOpen = !prompt.isPromptOpen
				gtx.Execute(op.InvalidateCmd{})
			} else if prompt.promptConfirm.Clicked(gtx) {
				prompt.onConfirm()
				prompt.isPromptOpen = !prompt.isPromptOpen
				gtx.Execute(op.InvalidateCmd{})
			}
			// Give some padding to box and lay it out
			return layout.UniformInset(unit.Dp(6)).Layout(gtx,
				func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						// Popup message
						layout.Flexed(0.5, func(gtx C) D {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								// Icon
								layout.Rigid(func(gtx C) D {
									img := widget.Image{Src: paint.NewImageOp(errorIco)}
									img.Position = layout.Center
									return img.Layout(gtx)
								}),
								// Spacer
								layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
								// Message label
								layout.Flexed(0.5, func(gtx C) D {
									c := gtx.Constraints
									c.Min.Y, c.Max.Y = 18, 18
									gtx.Constraints = c
									lbl := material.Label(th, unit.Sp(16), "Confirm deletion of 1 note item?")
									lbl.Font.Weight = font.Medium
									return lbl.Layout(gtx)
								}),
							)
						}),
						// Spacer
						layout.Rigid(layout.Spacer{Height: unit.Dp(30)}.Layout),
						// Action buttons
						layout.Rigid(func(gtx C) D {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								// Spacer to push below widgets to right
								layout.Flexed(0.5, func(gtx C) D {
									c := gtx.Constraints
									return layout.Dimensions{Size: image.Pt(c.Max.X, c.Min.Y)}
								}),
								// OK action
								layout.Rigid(material.Button(th, &prompt.promptConfirm, "Confirm").Layout),
								// Spacer
								layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
								// Cancel action
								layout.Rigid(func(gtx C) D {
									th_ := *th
									th_.Palette.ContrastBg = color.NRGBA{A: 0}
									return material.Button(&th_, &prompt.promptCancel, "Cancel").Layout(gtx)
								}),
							)
						}),
					)
				},
			)
		},
	)
	call := macro.Stop()
	gtx.Constraints = savedConstraints
	max := gtx.Constraints.Max
	x := math.Round(float64(max.X)/2 - float64(dims.Size.X)/2)
	y := math.Round(float64(max.Y)/2 - float64(dims.Size.Y)/2)
	defer op.Offset(image.Pt(int(x), int(y))).Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

func Layout(gtx C) D {
	dims := layout.Background{}.Layout(gtx,
		// Set a background
		func(gtx C) D {
			defer clip.Rect{Max: gtx.Constraints.Min}.Push(gtx.Ops).Pop()
			paint.ColorOp{Color: color.NRGBA{40, 40, 43, 255}}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			return layout.Dimensions{Size: gtx.Constraints.Min}
		},
		// Layout the content
		func(gtx C) D {
			return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx C) D {
				return layout.Flex{
					Axis: layout.Vertical,
				}.Layout(gtx,
					// Logo
					layout.Rigid(func(gtx C) D {
						return widget.Image{Src: paint.NewImageOp(logo)}.Layout(gtx)
					}),
					// Spacer
					layout.Rigid(layout.Spacer{Height: unit.Dp(14)}.Layout),
					// Layout the notesPane and editorPane
					layout.Flexed(0.5, func(gtx C) D {
						var children []layout.FlexChild
						children = append(children,
							layout.Rigid(notesPane),
							layout.Rigid(func(gtx C) D {
								return layout.Spacer{Width: unit.Dp(7)}.Layout(gtx)
							}),
						)
						if editorOpen {
							children = append(children, layout.Flexed(0.5, editorPane))
						}
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
					}),
					// Spacer
					layout.Rigid(layout.Spacer{Height: unit.Dp(7)}.Layout),
					// Layout message box
					layout.Rigid(msg.layout),
				)
			})
		},
	)
	// Trigger popup window if requested pop up
	if prompt.isPromptOpen {
		prompt.layout(gtx)
	}
	return dims
}

// The very first thing called in UI(); check for any available notes and load them
func Load() {
	f, err := os.Open(NOTES_SAVE_PATH)
	if err != nil && os.IsNotExist(err) {
		return
	} else {
		defer f.Close()
		data, _ := io.ReadAll(f)
		v := map[int]interface{}{}
		json.Unmarshal(xorEncryptDecrypt(data), &v)
		for _, val := range v {
			for key, val1 := range val.(map[string]interface{}) {
				notes = append(notes, note{title: key, content: val1.(string)})
				break
			}
		}
	}
}

func Save() {
	v := map[int]interface{}{}
	if len(scratchNotes) != 0 {
		for i, vv := range scratchNotes {
			v[i] = map[string]string{
				vv.title: vv.content,
			}
		}
	} else {
		for i, vv := range notes {
			v[i] = map[string]string{
				vv.title: vv.content,
			}
		}
	}
	_, err := os.Stat(NOTES_SAVE_DIR)
	if err != nil && os.IsNotExist(err) {
		os.Mkdir(NOTES_SAVE_DIR, os.ModePerm)
	}
	data, err := json.Marshal(v)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	f, err := os.OpenFile(NOTES_SAVE_PATH, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	defer f.Close()
	f.Write(xorEncryptDecrypt(data))
}

func xorEncryptDecrypt(input []byte) []byte {
	output := make([]byte, len(input))
	for i := range input {
		output[i] = input[i] ^ XOR_KEY[i%len(XOR_KEY)]
	}
	return output
}
