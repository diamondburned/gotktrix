// Package assistant provides an implementation of gtk.Assistant but
// mobile-friendly.
package assistant

import (
	"context"
	"fmt"
	"log"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"
)

// Assistant is a widget that behaves similarly to gtk.Assistant.
type Assistant struct {
	*gtk.Window
	bar *gtk.HeaderBar

	content *gtk.Box
	bread   *gtk.Box
	main    *gtk.Stack
	spin    *gtk.Spinner

	hscroll   *gtk.ScrolledWindow
	hviewport *gtk.Viewport

	cancel *gtk.Button
	ok     *gtk.Button

	cancelState struct {
		cancel  context.CancelFunc
		pressed func()
	}

	steps   []*Step
	current int
}

// Show is a helper function for showing an Assistant dialog immediately.
func Show(parent *gtk.Window, title string, steps []*Step) {
	a := New(parent, steps)
	a.SetTitle(title)
	a.Show()
}

var assistantBreadCSS = cssutil.Applier("assistant-bread", `
	.assistant-bread {
		padding: 10px 15px;
		box-shadow:
			0px -5px 6px -5px rgba(0, 0, 0, 0.1) inset,
			0px  5px 6px -5px rgba(0, 0, 0, 0.1) inset;
		background-color: mix(@theme_bg_color, @theme_fg_color, 0.1);
	}
`)

// New creates a new Assistant.
func New(parent *gtk.Window, steps []*Step) *Assistant {
	bread := gtk.NewBox(gtk.OrientationHorizontal, 5)
	bread.SetHExpand(true)
	assistantBreadCSS(bread)

	hviewport := gtk.NewViewport(nil, nil)
	hviewport.SetFocusOnClick(false)
	hviewport.SetScrollToFocus(true)
	hviewport.SetChild(bread)

	hscroll := gtk.NewScrolledWindow()
	hscroll.SetPolicy(gtk.PolicyAutomatic, gtk.PolicyNever)
	hscroll.SetChild(hviewport)

	spinner := gtk.NewSpinner()
	spinner.SetVAlign(gtk.AlignCenter)
	spinner.SetHAlign(gtk.AlignCenter)
	spinner.SetSizeRequest(64, 64)

	stack := gtk.NewStack()
	stack.AddChild(spinner)

	content := gtk.NewBox(gtk.OrientationVertical, 0)
	content.SetHExpand(true)
	content.SetVExpand(true)
	content.Append(hscroll)
	content.Append(stack)

	cancel := gtk.NewButtonWithLabel("Close")
	ok := gtk.NewButtonWithLabel("OK")

	bar := gtk.NewHeaderBar()
	bar.SetTitleWidget(gtk.NewLabel(""))
	bar.SetShowTitleButtons(false)
	bar.PackStart(cancel)
	bar.PackEnd(ok)

	window := gtk.NewWindow()
	window.SetDefaultSize(400, 500)
	window.SetTransientFor(parent)
	window.SetModal(true)
	window.SetTitlebar(bar)
	window.SetChild(content)

	assistant := Assistant{
		Window:    window,
		content:   content,
		bread:     bread,
		hscroll:   hscroll,
		hviewport: hviewport,
		main:      stack,
		spin:      spinner,
		cancel:    cancel,
		ok:        ok,
		current:   -1,
	}

	cancel.Connect("clicked", func() {
		if assistant.cancelState.cancel != nil {
			assistant.cancelState.cancel()
			assistant.cancel.SetSensitive(false)
			return
		}

		if assistant.cancelState.pressed != nil {
			assistant.cancelState.pressed()
			return
		}

		// 		if assistant.current > 0 && assistant.current < len(assistant.steps) {
		// 			step := assistant.steps[assistant.current]
		// 			if step.CanBack {
		// 				assistant.SetIndex(assistant.current - 1)
		// 				return
		// 			}
		// 		}

		window.Close()
	})

	ok.Connect("clicked", func() {
		if assistant.current < 0 {
			return
		}

		currentStep := assistant.steps[assistant.current]
		if currentStep.Done != nil {
			currentStep.Done(currentStep)
			return
		}

		if assistant.current == len(assistant.steps)-1 {
			// Last step; close window.
			window.Close()
		} else {
			// Not last step; move on.
			assistant.SetIndex(assistant.current + 1)
		}
	})

	for _, step := range steps {
		assistant.AddStep(step)
	}

	assistant.restoreStackTransitions()

	return &assistant
}

// OKButton returns the OK button.
func (a *Assistant) OKButton() *gtk.Button { return a.ok }

// CancelButton returns the Cancel button.
func (a *Assistant) CancelButton() *gtk.Button { return a.cancel }

// Busy marks the dialog as busy and shows the spinner.
func (a *Assistant) Busy() {
	a.busy(false)
}

// CancellableBusy marks the dialog as busy but allows the Cancel button to stop
// the context. The caller must call Continue() to restore the dialog.
func (a *Assistant) CancellableBusy(parent context.Context) context.Context {
	a.busy(true)

	ctx, cancel := context.WithCancel(parent)
	a.cancelState.cancel = cancel
	a.cancel.SetLabel("Cancel")

	return ctx
}

func (a *Assistant) busy(keepCancel bool) {
	a.main.SetTransitionDuration(50)
	a.main.SetTransitionType(gtk.StackTransitionTypeCrossfade)

	a.main.SetSensitive(false)
	a.main.SetVisibleChild(a.spin)
	a.spin.Start()

	a.ok.SetSensitive(false)
	a.cancel.SetSensitive(keepCancel)
}

// Continue restores the dialog and brings it out of busy mode.
func (a *Assistant) Continue() {
	a.SetIndex(a.current)
}

func (a *Assistant) restoreStackTransitions() {
	a.main.SetTransitionDuration(75)
	a.main.SetTransitionType(gtk.StackTransitionTypeSlideLeftRight)
}

// AddNewStep creates a new step to be added into the assistant. The new step is
// returned.
func (a *Assistant) AddNewStep(title, okLabel string) *Step {
	step := NewStep(title, okLabel)
	a.AddStep(step)
	return step
}

// AddStep adds a step into the assistant.
func (a *Assistant) AddStep(step *Step) {
	if step.parent != nil {
		log.Println("given step is already added elsewhere")
		return
	}

	step.parent = a
	step.index = len(a.steps)
	a.steps = append(a.steps, step)

	log.Println("adding step", step.index)

	arrow := gtk.NewLabel("⟩")
	arrow.SetCSSClasses([]string{"assistant-crumb", "assistant-crumb-arrow"})
	arrow.SetAttributes(crumbArrowAttrs)
	breadcrumbCSS(arrow)

	a.main.AddTitled(step.content, fmt.Sprintf("page-%d", step.index), step.title)
	a.bread.Append(arrow)
	a.bread.Append(step.titleLabel)

	if a.current == -1 {
		a.SetStep(step)
	}
}

// NextStep moves the assistant to the next step. It panics if the assistant is
// currently at the last step.
func (a *Assistant) NextStep() {
	a.SetIndex(a.current + 1)
}

// SetIndex sets the assistant to the step at the given index.
func (a *Assistant) SetIndex(ix int) {
	a.SetStep(a.steps[ix])
}

// SetStep sets the assistant to show the given step. If the Assistant is
// currently busy, then it'll continue.
func (a *Assistant) SetStep(step *Step) {
	if step.parent != a {
		log.Println("given step does not belong to assistant")
		return
	}

	if a.cancelState.cancel != nil {
		a.cancelState.cancel()
		a.cancelState.cancel = nil
	}

	a.main.SetSensitive(true)
	a.ok.SetSensitive(true)
	a.cancel.SetSensitive(true)

	a.current = step.index

	a.main.SetVisibleChild(step.content)
	a.ok.SetLabel(step.okLabel)
	a.ok.SetVisible(step.okLabel != "") // hide if label is empty

	// if step.CanBack && a.current > 0 {
	// 	a.cancel.SetLabel("Back")
	// } else {
	// 	a.cancel.SetLabel("Close")
	// }

	a.updateBreadcrumb()
	a.restoreStackTransitions()
}

var (
	crumbInactiveClass = []string{"assistant-crumb"}
	crumbActiveClass   = []string{"assistant-crumb", "assistant-active-crumb"}

	crumbInactiveAttrs = markuputil.Attrs(
		pango.NewAttrScale(1.1),
		pango.NewAttrWeight(pango.WeightBook),
	)
	crumbActiveAttrs = markuputil.Attrs(
		pango.NewAttrScale(1.3),
		pango.NewAttrWeight(pango.WeightBold),
	)
	crumbArrowAttrs = markuputil.Attrs(
		pango.NewAttrScale(1.2),
		pango.NewAttrWeight(pango.WeightBold),
	)
)

var breadcrumbCSS = cssutil.Applier(crumbInactiveClass[0], `
	.assistant-crumb {
		color: alpha(@theme_fg_color, 0.5);
		transition: linear 75ms color;
	}
	
	.assistant-crumb-arrow {
		color: alpha(@theme_fg_color, 0.25);
	}
	
	.assistant-active-crumb {
		color: @accent_bg_color;
	}
`)

func (a *Assistant) updateBreadcrumb() {
	for i := 0; i < a.current; i++ {
		a.steps[i].titleLabel.SetCSSClasses(crumbInactiveClass)
		a.steps[i].titleLabel.SetAttributes(crumbInactiveAttrs)
	}
	for i := a.current + 1; i < len(a.steps); i++ {
		a.steps[i].titleLabel.SetCSSClasses(crumbInactiveClass)
		a.steps[i].titleLabel.SetAttributes(crumbInactiveAttrs)
	}
	a.steps[a.current].titleLabel.SetCSSClasses(crumbActiveClass)
	a.steps[a.current].titleLabel.SetAttributes(crumbActiveAttrs)

	labelRect := a.steps[a.current].titleLabel.Allocation()

	// Scroll the breadcrumb to the end.
	hadj := a.hviewport.HAdjustment()
	hadj.SetValue(hadj.Upper() - hadj.PageSize() - float64(labelRect.X()))

	// Scroll the breadcrumb back to the child. This way, the user can easily
	// see what goes after the current crumb.
	// a.hviewport.SetFocusChild(a.steps[a.current].titleLabel)
}

// Step describes each step of the assistant.
type Step struct {
	// Done is called when the OK button is clicked and there isn't a next page.
	Done func(*Step)
	// CanBack, if true, will allow the assistant to go from this step to the
	// last step. If this is the first step, then this does nothing.
	// CanBack bool

	parent     *Assistant
	content    *gtk.Box
	titleLabel *gtk.Label

	title   string
	okLabel string

	index int
}

// StepData is the primitive data of a step that is used to build steps.
type StepData struct {
	Title    string
	OKLabel  string
	Contents []gtk.Widgetter
	Done     func(*Step)
}

// NewStepData builds a new step data.
func NewStepData(title, okLabel string, contents ...gtk.Widgetter) StepData {
	return StepData{title, okLabel, contents, nil}
}

// BuildSteps builds multiple steps using the given data.
func BuildSteps(data ...StepData) []*Step {
	steps := make([]*Step, len(data))
	for i, data := range data {
		steps[i] = NewStep(data.Title, data.OKLabel)
		steps[i].Done = data.Done
		for _, widget := range data.Contents {
			steps[i].content.Append(widget)
		}
	}
	return steps
}

var stepBodyCSS = cssutil.Applier("assistant-stepbody", `
	.assistant-stepbody {
		padding: 10px 15px;
	}
`)

// NewStep creates a new assistant step.
func NewStep(title, okLabel string) *Step {
	content := gtk.NewBox(gtk.OrientationVertical, 0)
	content.SetHExpand(true)
	content.SetVExpand(true)
	content.SetHAlign(gtk.AlignCenter)
	content.SetVAlign(gtk.AlignCenter)
	stepBodyCSS(content)

	titleLabel := gtk.NewLabel(title)
	titleLabel.SetAttributes(crumbInactiveAttrs)
	breadcrumbCSS(titleLabel)

	return &Step{
		content:    content,
		titleLabel: titleLabel,
		title:      title,
		okLabel:    okLabel,
	}
}

// ContentArea returns the underlying step's content area box.
func (step *Step) ContentArea() *gtk.Box {
	return step.content
}

// Assistant returns the step's parent assistant. Nil is returned if the
// Assistant is a zero-value.
func (step *Step) Assistant() *Assistant {
	return step.parent
}
