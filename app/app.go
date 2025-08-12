package app

import (
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

	"github.com/deoxyimran/keeper/app/utils/svgs"
	"github.com/deoxyimran/keeper/res/images"

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

type App struct {
	// Widgets
	notesPane  notesPane
	editorPane editorPane
	notif      notification
	prompt     msgPrompt
	// States
	scratchNotes []note
	notes        []note
	selectedNote int
	isEditorOpen bool
	// Logo, theme, etc.
	logo image.Image
	th   *material.Theme
}

type note struct {
	title, content string
	isSelected     bool
	isHovered      bool
}

type (
	C = layout.Context
	D = layout.Dimensions
)

const (
	NOTES_SAVE_PATH = "data/notes.bin"
	NOTES_SAVE_DIR  = "data"
	XOR_KEY         = "k@@P*Robfuscated"
	IMG_PATH        = "res/images/"
)

func NewApp() *App {
	app := &App{}

	// Loading app logo and icons
	app.logo, _ = png.Decode(bytes.NewReader(images.Logo))
	noteIco, _ := svgs.LoadSvg(strings.NewReader(images.Note), image.Point{})
	searchIco, _ := svgs.LoadSvg(strings.NewReader(images.Search), image.Point{})
	trashIco, _ := svgs.LoadSvg(strings.NewReader(images.Trash), image.Point{})
	errorIco, _ := svgs.LoadSvg(strings.NewReader(images.Error), image.Point{48, 48})
	xcircleIco, _ := svgs.LoadSvg(strings.NewReader(images.XCircle), image.Pt(18, 18))

	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	th.Fg = color.NRGBA{250, 249, 246, 255}
	app.th = th

	// Init prompt
	app.prompt = newMsgPrompt(th, 350, 140, errorIco)

	// Init notification
	app.notif = newNotification(th, xcircleIco)

	// Panes
	app.notesPane = newNotesPane(th, searchIco, noteIco, &app.scratchNotes, &app.notes, &app.selectedNote, &app.isEditorOpen)
	app.editorPane = newEditorPane(th, trashIco, &app.prompt, &app.notif)

	return app

}

type button struct {
	th         *material.Theme
	clickable  widget.Clickable
	label      string
	isDisabled bool
	onClick    func()
}

func (a *button) update(gtx C) {
	if !a.isDisabled && a.clickable.Clicked(gtx) {
		if a.onClick != nil {
			a.onClick()
		}
	}
}

func (a *button) layout(gtx C) D {
	a.update(gtx)
	btn := material.Button(a.th, &a.clickable, a.label)
	dims := btn.Layout(gtx)
	if a.isDisabled {
		defer clip.UniformRRect(image.Rect(0, 0, dims.Size.X, dims.Size.Y), 3).Push(gtx.Ops).Pop()
		paint.ColorOp{Color: color.NRGBA{B: 200, A: 190}}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		event.Op(gtx.Ops, a)
	}
	return dims
}

type noteItem struct {
	th             *material.Theme
	ico            image.Image
	get            func(i int) *note
	getSelectedInd func() int
	isSelected     func(i int) bool
	isHovered      func(i int) bool
	handleHover    func(i int)
	handleUnhover  func(i int)
	handleSelect   func(i int)
	handleUnselect func(i int)
}

func (ni *noteItem) layout(gtx C, index int) D {
	macro := op.Record(gtx.Ops)
	f := func(gtx C) D {
		return layout.Flex{
			Axis:      layout.Horizontal,
			Alignment: layout.Middle,
		}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return widget.Image{Src: paint.NewImageOp(ni.ico)}.Layout(gtx)
			}),
			layout.Flexed(0.5, func(gtx C) D {
				return material.Label(ni.th, unit.Sp(13), ni.get(index).title).Layout(gtx)
			}),
		)
	}
	var dims layout.Dimensions

	if !ni.isSelected(index) && ni.isHovered(index) {
		// Hovered state
		dims = layout.Background{}.Layout(gtx,
			func(gtx C) D {
				x, y := gtx.Constraints.Min.X, gtx.Constraints.Min.Y
				defer clip.UniformRRect(image.Rect(0, 0, x, y), 10).Push(gtx.Ops).Pop()
				paint.ColorOp{Color: color.NRGBA{B: 155, A: 90}}.Add(gtx.Ops)
				paint.PaintOp{}.Add(gtx.Ops)
				return D{Size: image.Pt(x, y)}
			}, f,
		)
	} else if ni.isSelected(index) {
		// Selected state
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
	event.Op(gtx.Ops, ni.get(index))
	// Check for events and select
	for {
		ev, ok := gtx.Source.Event(pointer.Filter{
			Target: ni.get(index),
			Kinds:  pointer.Press | pointer.Release | pointer.Enter | pointer.Leave,
		})
		if !ok {
			break
		}
		if x, ok := ev.(pointer.Event); ok {
			switch x.Kind {
			case pointer.Enter:
				ni.handleHover(index)
			case pointer.Release:
				ni.handleUnhover(index)
			case pointer.Press:
				ni.handleUnselect(ni.getSelectedInd())
				ni.handleSelect(index)
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

type notesPane struct {
	// Widget
	th         *material.Theme
	searchIco  image.Image
	addNoteBtn button
	noteItem   noteItem
	notesListW widget.List
	searchBarW widget.Editor
	// States
	scratchNotes *[]note
	notes        *[]note
	selectedNote *int
	isEditorOpen *bool
}

func newNotesPane(th *material.Theme, searchIco image.Image, noteIco image.Image, scratchNotes *[]note,
	notes *[]note, selectedNote *int, isEditorOpen *bool) notesPane {

	np := notesPane{
		th:           th,
		scratchNotes: scratchNotes,
		notes:        notes,
		selectedNote: selectedNote,
		isEditorOpen: isEditorOpen,
		searchIco:    searchIco,
	}
	np.noteItem = noteItem{ // init note item
		th:  th,
		ico: noteIco,
		//Assign funcs
		get:            np.getNote,
		getSelectedInd: np.getSelectedNoteInd,
		isSelected:     np.isNoteSelected,
		isHovered:      np.isNoteHovered,
		handleHover:    np.handleHoverNote,
		handleUnhover:  np.handleUnhoverNote,
		handleSelect:   np.handleSelectNote,
		handleUnselect: np.handleUnselectNote,
	}
	return np
}

func (np *notesPane) getNote(i int) *note {
	return &(*np.notes)[i]
}

func (np *notesPane) getSelectedNoteInd() int {
	return *np.selectedNote
}

func (np *notesPane) isNoteSelected(i int) bool {
	return (*np.notes)[i].isSelected
}

func (np *notesPane) isNoteHovered(i int) bool {
	return (*np.notes)[i].isHovered
}

func (np *notesPane) handleHoverNote(i int) {
	(*np.notes)[i].isHovered = true
}

func (np *notesPane) handleUnhoverNote(i int) {
	(*np.notes)[i].isHovered = false
}

func (np *notesPane) handleSelectNote(i int) {
	*np.selectedNote = i
	(*np.notes)[i].isSelected = true
	*np.isEditorOpen = true
}

func (np *notesPane) handleUnselectNote(i int) {
	*np.selectedNote = -1
	(*np.notes)[i].isSelected = false
	*np.isEditorOpen = false
}

func (np *notesPane) searchNotes(query string) {
	query = strings.ToLower(query)
	*np.selectedNote = -1
	if len(*np.notes) == 0 && len(*np.scratchNotes) == 0 {
		return
	}
	var tempNotes []note
	if np.scratchNotes == nil {
		for _, v := range *np.notes {
			if strings.Contains(strings.ToLower(v.title), query) {
				tempNotes = append(tempNotes, v)
			}
		}
		*np.scratchNotes = *np.notes
	} else {
		for _, v := range *np.scratchNotes {
			if strings.Contains(strings.ToLower(v.title), query) {
				tempNotes = append(tempNotes, v)
			}
		}
	}
	*np.notes = tempNotes
}

func (np *notesPane) updateNotes(gtx C) {
	// Check search
	prevSearch := np.searchBarW.Text()
	if s := np.searchBarW.Text(); s != prevSearch {
		if s == "" {
			np.addNoteBtn.isDisabled = false
			if np.scratchNotes != nil {
				*np.notes = *np.scratchNotes
				np.scratchNotes = nil
			}
		} else {
			np.addNoteBtn.isDisabled = true
			*np.isEditorOpen = false
			if len(*np.notes) != 0 && *np.selectedNote != -1 {
				(*np.notes)[*np.selectedNote].isSelected = false
			}
			np.searchNotes(s)
		}
		gtx.Execute(op.InvalidateCmd{})
	}
}

func (np *notesPane) layout(gtx C) D {
	// Update notes
	np.updateNotes(gtx)

	w := 230
	gtx.Constraints.Max.X, gtx.Constraints.Min.X = w, w // hardcode width

	return layout.Flex{
		Axis: layout.Vertical,
	}.Layout(gtx,
		// Searchbar
		layout.Rigid(func(gtx C) D {
			// Layout searchbar
			dims := layout.Background{}.Layout(gtx,
				// Layout searchbar bg
				func(gtx C) D {
					sz := gtx.Constraints.Min
					defer clip.UniformRRect(image.Rect(0, 0, sz.X, sz.Y), 5).Push(gtx.Ops).Pop()
					paint.ColorOp{Color: color.NRGBA{255, 255, 255, 20}}.Add(gtx.Ops)
					paint.PaintOp{}.Add(gtx.Ops)
					return layout.Dimensions{Size: sz}
				},
				// Layout searchbar
				func(gtx C) D {
					return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx C) D {
						edit := material.Editor(np.th, &np.searchBarW, "Search")
						edit.Font.Style = font.Italic
						edit.TextSize = unit.Sp(14)
						img := widget.Image{Src: paint.NewImageOp(np.searchIco)}
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
			return dims
		}),
		// Spacer
		layout.Rigid(layout.Spacer{Height: unit.Dp(7)}.Layout),
		// Layout note items
		layout.Flexed(0.5, func(gtx C) D {
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
						return material.List(np.th, &np.notesListW).Layout(gtx, len(*np.notes), np.noteItem.layout)
					})
				},
			)
		}),
		// Spacer
		layout.Rigid(layout.Spacer{Height: unit.Dp(7)}.Layout),
		// 'Add Note' button
		layout.Rigid(func(gtx C) D {
			if np.addNoteBtn.clickable.Clicked(gtx) {
				*np.notes = append(*np.notes, note{title: "Untitled"})
			}
			return np.addNoteBtn.layout(gtx)
		}),
	)
}

type icoButton struct {
	ico     image.Image
	onClick func()
}

func (i *icoButton) layout(gtx C) D {
	macro := op.Record(gtx.Ops)
	dims := widget.Image{Src: paint.NewImageOp(i.ico)}.Layout(gtx)
	call := macro.Stop()
	// Register and check for events
	defer clip.Rect{Max: dims.Size}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, &i)
	for {
		ev, ok := gtx.Source.Event(pointer.Filter{
			Target: &i.ico,
			Kinds:  pointer.Press | pointer.Release,
		})
		if !ok {
			break
		}
		if x, ok := ev.(pointer.Event); ok {
			switch x.Kind {
			case pointer.Release:
				if i.onClick != nil {
					i.onClick()
				}
				gtx.Execute(op.InvalidateCmd{})
			}
		}
	}
	// Layout the widget
	call.Add(gtx.Ops)
	return dims
}

type editorPane struct {
	// Wigets
	th          *material.Theme
	prompt      *msgPrompt
	notif       *notification
	trashBtn    icoButton
	titleEditor widget.Editor
	noteEditor  widget.Editor
	// States
	scratchNotes *[]note
	notes        *[]note
	selectedNote *int
	isEditorOpen *bool
}

func newEditorPane(th *material.Theme, trashIco image.Image, prompt *msgPrompt, notif *notification) editorPane {
	e := editorPane{
		th:     th,
		prompt: prompt,
		notif:  notif,
		trashBtn: icoButton{
			ico: trashIco,
		},
	}
	return e
}

func (e *editorPane) layout(gtx C) D {
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
					prevTitle := e.titleEditor.Text()
					// Layout everything
					edit := material.Editor(e.th, &e.titleEditor, "Title")
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
					if s := e.titleEditor.Text(); s != prevTitle {
						(*e.notes)[*e.selectedNote].title = s
						gtx.Execute(op.InvalidateCmd{})
					}
					return dims
				}),
				// Spacer
				layout.Rigid(layout.Spacer{Width: unit.Dp(5)}.Layout),
				// Trash button
				layout.Rigid(func(gtx C) D {
					e.trashBtn.onClick = func() {
						e.prompt.onConfirm = func() {
							// Delete note pointed to by currentInd
							t := (*e.notes)[*e.selectedNote].title
							c := (*e.notes)[*e.selectedNote].content
							*e.notes = slices.Delete(*e.notes, *e.selectedNote, *e.selectedNote+1)
							for i := range *e.scratchNotes {
								b := strings.Contains((*e.scratchNotes)[i].title, t) &&
									strings.Contains((*e.scratchNotes)[i].content, c)
								if b {
									*e.scratchNotes = slices.Delete(*e.scratchNotes, i, i+1)
									break
								}
							}
							*e.isEditorOpen = !*e.isEditorOpen
							*e.selectedNote = -1
							// e.prompt.resetOffset()
							// e.prompt.isAnimating = true
							// e.prompt.isPromptOpen = !e.prompt.isPromptOpen
						}
					}
					return e.trashBtn.layout(gtx)
				}),
			)
		}),
		// Spacer
		layout.Rigid(layout.Spacer{Height: unit.Dp(7)}.Layout),
		// Note editor
		layout.Flexed(0.5, func(gtx C) D {
			// Get the last note text
			prevNote := e.noteEditor.Text()
			// Layout everything
			edit := material.Editor(e.th, &e.noteEditor, "Write something...")
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
			if s := e.noteEditor.Text(); s != prevNote {
				(*e.notes)[*e.selectedNote].content = s
				gtx.Execute(op.InvalidateCmd{})
			}
			return dims
		}),
	)
}

type notification struct {
	th          *material.Theme
	xcircleIco  image.Image
	isAnimating bool
	offsetY     float32
}

func newNotification(th *material.Theme, xcircleIco image.Image) notification {
	return notification{
		th:         th,
		xcircleIco: xcircleIco,
	}
}

func (nf *notification) layout(gtx C) D {
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
					th_ := *nf.th
					th_.Fg = color.NRGBA{23, 27, 23, 255}
					lbl := material.Label(&th_, unit.Sp(14), "Successfully deleted note!")
					lbl.Font.Weight = font.ExtraBold
					lbl.Alignment = text.Middle
					return lbl.Layout(cgtx)
				}),
				// Cross button to hide notification from view
				layout.Rigid(func(gtx C) D {
					img := widget.Image{Src: paint.NewImageOp(nf.xcircleIco)}
					img.Position = layout.Center
					macro := op.Record(gtx.Ops)
					dims := img.Layout(gtx)
					call := macro.Stop()
					// Tag event area
					defer clip.Rect{Max: dims.Size}.Push(gtx.Ops).Pop()
					event.Op(gtx.Ops, &nf.xcircleIco)
					// Check for events
					for {
						ev, ok := gtx.Source.Event(pointer.Filter{
							Target: &nf.xcircleIco,
							Kinds:  pointer.Press | pointer.Release,
						})
						if !ok {
							break
						}
						if x, ok := ev.(pointer.Event); ok {
							switch x.Kind {
							case pointer.Release:
								// msg.resetOffset()
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

	// Mark event area
	defer clip.Rect{Max: dims.Size}.Push(gtx.Ops).Pop()

	if nf.isAnimating { // Animate
		nf.offsetY += 0.7
		if nf.offsetY > float32(dims.Size.Y) {
			nf.offsetY = float32(dims.Size.Y)
			nf.isAnimating = false
		}
		gtx.Execute(op.InvalidateCmd{})
		offset := float32(dims.Size.Y) - nf.offsetY
		if offset <= 0.005 {
			offset = 0
		}
		defer op.Offset(image.Pt(0, int(offset))).Push(gtx.Ops).Pop()
		call.Add(gtx.Ops)
	} else {
		nf.offsetY = 0 // Reset offset when not animating
		call.Add(gtx.Ops)
	}

	return dims
}

func (nf *notification) show() {
	nf.isAnimating = true
}

type msgPrompt struct {
	th                                *material.Theme
	errorIco                          image.Image
	onConfirm                         func()
	cancelClickable, confirmClickable widget.Clickable
	isPromptOpen                      bool
	backdropTag                       int
	w, h                              int
}

func newMsgPrompt(th *material.Theme, w, h int, errorIco image.Image) msgPrompt {
	return msgPrompt{
		th:       th,
		w:        w,
		h:        h,
		errorIco: errorIco,
	}
}

func (p *msgPrompt) open() {
	p.isPromptOpen = true
}

func (p *msgPrompt) close() {
	p.isPromptOpen = false
}

func (p *msgPrompt) layout(gtx C) D {
	// Set a backdrop
	trans := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
	paint.ColorOp{Color: color.NRGBA{A: 180}}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	event.Op(gtx.Ops, &p.backdropTag)
	trans.Pop()
	// Save constraints and resize constraints min max
	pt := image.Pt(p.w, p.h)
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
			if p.cancelClickable.Clicked(gtx) {
				p.isPromptOpen = !p.isPromptOpen
				gtx.Execute(op.InvalidateCmd{})
			} else if p.confirmClickable.Clicked(gtx) {
				p.onConfirm()
				p.isPromptOpen = !p.isPromptOpen
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
									img := widget.Image{Src: paint.NewImageOp(p.errorIco)}
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
									lbl := material.Label(p.th, unit.Sp(16), "Confirm deletion of 1 note item?")
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
								layout.Rigid(material.Button(p.th, &p.confirmClickable, "Confirm").Layout),
								// Spacer
								layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
								// Cancel action
								layout.Rigid(func(gtx C) D {
									th_ := *p.th
									th_.Palette.ContrastBg = color.NRGBA{A: 0}
									return material.Button(&th_, &p.cancelClickable, "Cancel").Layout(gtx)
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

func (a *App) Layout(gtx C) D {
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
						return widget.Image{Src: paint.NewImageOp(a.logo)}.Layout(gtx)
					}),
					// Spacer
					layout.Rigid(layout.Spacer{Height: unit.Dp(14)}.Layout),
					// Layout the notesPane and editorPane
					layout.Flexed(0.5, func(gtx C) D {
						var children []layout.FlexChild
						children = append(children,
							layout.Rigid(a.notesPane.layout),
							layout.Rigid(func(gtx C) D {
								return layout.Spacer{Width: unit.Dp(7)}.Layout(gtx)
							}),
						)
						if a.isEditorOpen {
							children = append(children, layout.Flexed(0.5, a.editorPane.layout))
						}
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
					}),
					// Spacer
					layout.Rigid(layout.Spacer{Height: unit.Dp(7)}.Layout),
					// Layout notification
					layout.Rigid(a.notif.layout),
				)
			})
		},
	)
	// Trigger prompt if requested
	if a.prompt.isPromptOpen {
		a.prompt.layout(gtx)
	}
	return dims
}

// The very first thing called in UI(); check for any available notes and load them
func (a *App) Load() {
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
				a.notes = append(a.notes, note{title: key, content: val1.(string)})
				break
			}
		}
	}
}

func (a *App) Save() {
	v := map[int]interface{}{}
	if len(a.scratchNotes) != 0 {
		for i, vv := range a.scratchNotes {
			v[i] = map[string]string{
				vv.title: vv.content,
			}
		}
	} else {
		for i, vv := range a.notes {
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
