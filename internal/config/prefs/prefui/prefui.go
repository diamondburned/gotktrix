package prefui

import (
	"context"
	"log"
	"math"
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/config"
	"github.com/diamondburned/gotktrix/internal/config/prefs"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/locale"
	"github.com/pkg/errors"
)

// Dialog is a widget that lists all known preferences in a dialog.
type Dialog struct {
	*gtk.Dialog
	ctx context.Context

	header   *gtk.HeaderBar
	box      *gtk.Box
	search   *gtk.SearchBar
	loading  *gtk.Spinner
	sections []*section

	saver config.ConfigStore
}

var currentDialog *Dialog

// ShowDialog shows the preferences dialog.
func ShowDialog(ctx context.Context) {
	if currentDialog != nil {
		currentDialog.ctx = ctx
		currentDialog.Present()
		return
	}

	dialog := newDialog(ctx)
	dialog.ConnectClose(func() {
		currentDialog = nil
		dialog.Destroy()
	})
	dialog.Show()
}

var _ = cssutil.WriteCSS(`
	.prefui-section-box:not(:first-child) {
		border-top: 1px solid @borders;
	}

	.prefui-heading {
		margin: 16px;
		margin-bottom: 10px;

		font-weight: bold;
		font-size: 0.95em;

		color: mix(@theme_fg_color, @theme_bg_color, 0.15);
	}

	row.prefui-prop, list.prefui-section {
		border: none;
		background: none;
	}

	.prefui-section {
		margin:  0 10px;
		padding: 0;
	}

	.prefui-section > row {
		margin: 8px 4px;
	}

	.prefui-section > row,
	.prefui-section > row:hover,
	.prefui-section > row:active {
		background: none;
	}

	.prefui-section > row:not(:first-child) {
		border-top: 2px solid @theme_bg_color;
	}

	.prefui-prop > box.vertical > .prefui-prop {
		margin-top: 6px;
	}

	.prefui-prop > box.horizontal > .prefui-prop {
		margin-left: 6px;
	}

	.prefui-prop-description {
		font-size: 0.9em;
		color: mix(@theme_fg_color, @theme_bg_color, 0.15);
	}

	.prefui-prop-string {
		font-size: 0.9em;
	}
`)

func configSnapshotter(ctx context.Context) func() (save func()) {
	return func() func() {
		snapshot := prefs.TakeSnapshot()
		return func() {
			if err := snapshot.Save(); err != nil {
				app.Error(ctx, errors.Wrap(err, "cannot save prefs"))
			}
		}
	}
}

// newDialog creates a new preferences UI.
func newDialog(ctx context.Context) *Dialog {
	d := Dialog{ctx: ctx}

	d.saver = config.NewConfigStore(configSnapshotter(ctx))
	d.saver.Widget = (*dialogSaver)(&d)
	// Computers are just way too fast. Ensure that the loading circle visibly
	// pops up before it closes.
	d.saver.Minimum = 100 * time.Millisecond

	d.box = gtk.NewBox(gtk.OrientationVertical, 0)

	sections := prefs.ListProperties(ctx)
	d.sections = make([]*section, len(sections))
	for i, section := range sections {
		d.sections[i] = newSection(&d, section)
		d.box.Append(d.sections[i])
	}

	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetVExpand(true)
	scroll.SetChild(d.box)

	searchEntry := gtk.NewSearchEntry()
	searchEntry.SetObjectProperty("placeholder-text", locale.S(ctx, "Search Preferences..."))
	searchEntry.ConnectSearchChanged(func() { d.Search(searchEntry.Text()) })

	d.search = gtk.NewSearchBar()
	d.search.SetChild(searchEntry)
	d.search.ConnectEntry(&searchEntry.Editable)

	searchButton := gtk.NewToggleButton()
	searchButton.SetIconName("system-search-symbolic")
	searchButton.ConnectClicked(func() {
		d.search.SetSearchMode(searchButton.Active())
	})
	d.search.Connect("notify::search-mode-enabled", func() {
		searchButton.SetActive(d.search.SearchMode())
	})

	outerBox := gtk.NewBox(gtk.OrientationVertical, 0)
	outerBox.Append(d.search)
	outerBox.Append(scroll)

	d.Dialog = gtk.NewDialogWithFlags(
		locale.S(ctx, "Preferences"), app.GTKWindowFromContext(ctx),
		gtk.DialogDestroyWithParent|gtk.DialogUseHeaderBar,
	)
	d.Dialog.AddCSSClass("prefui-dialog")
	d.Dialog.SetModal(false)
	d.Dialog.SetDefaultSize(400, 500)
	d.Dialog.SetChild(outerBox)

	// Set this to the whole dialog instead of just the child.
	d.search.SetKeyCaptureWidget(d.Dialog)

	d.loading = gtk.NewSpinner()
	d.loading.SetSizeRequest(24, 24)
	d.loading.Hide()

	d.header = d.Dialog.HeaderBar()
	d.header.PackEnd(searchButton)
	d.header.PackEnd(d.loading)

	return &d
}

func (d *Dialog) Search(query string) {
	query = strings.ToLower(query)
	for _, section := range d.sections {
		section.Search(query)
	}
}

func (d *Dialog) save() {
	d.saver.Save()
}

type dialogSaver Dialog

func (d *dialogSaver) SaveBegin() {
	d.loading.Show()
	d.loading.Start()
}

func (d *dialogSaver) SaveEnd() {
	d.loading.Stop()
	d.loading.Hide()
}

type section struct {
	*gtk.Box
	name *gtk.Label
	list *gtk.ListBox

	props []*propRow

	searching string
	noResults bool
}

func newSection(d *Dialog, sect prefs.ListedSection) *section {
	s := section{}
	s.list = gtk.NewListBox()
	s.list.AddCSSClass("prefui-section")
	s.list.SetSelectionMode(gtk.SelectionNone)
	s.list.SetActivateOnSingleClick(true)

	s.props = make([]*propRow, len(sect.Props))
	for i, prop := range sect.Props {
		s.props[i] = newPropRow(d, prop)
		s.list.Append(s.props[i])
	}

	s.name = gtk.NewLabel(sect.Name)
	s.name.AddCSSClass("prefui-heading")
	s.name.SetXAlign(0)

	s.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	s.Box.AddCSSClass("prefui-section-box")
	s.Box.Append(s.name)
	s.Box.Append(s.list)

	s.list.SetFilterFunc(func(row *gtk.ListBoxRow) bool {
		prop := s.props[row.Index()]

		if strings.Contains(prop.queryTerm, s.searching) {
			s.noResults = false
			return true
		}

		return false
	})

	return &s
}

func (s *section) Search(query string) {
	s.noResults = true
	s.searching = query
	s.list.InvalidateFilter()
	// Hide if no results.
	s.SetVisible(!s.noResults)
}

type propRow struct {
	*gtk.ListBoxRow
	box *gtk.Box

	left struct {
		*gtk.Box
		name *gtk.Label
		desc *gtk.Label
	}
	action propWidget

	queryTerm string
}

func newPropRow(d *Dialog, prop prefs.LocalizedProp) *propRow {
	row := propRow{
		// Hacky way to do case-insensitive search.
		queryTerm: strings.ToLower(prop.Name) + strings.ToLower(prop.Description),
	}

	row.ListBoxRow = gtk.NewListBoxRow()
	row.AddCSSClass("prefui-prop")

	row.left.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	row.left.SetHExpand(true)

	row.left.name = gtk.NewLabel(prop.Name)
	row.left.name.AddCSSClass("prefui-prop-name")
	row.left.name.SetUseMarkup(true)
	row.left.name.SetXAlign(0)
	row.left.name.SetWrap(true)
	row.left.name.SetWrapMode(pango.WrapWordChar)
	row.left.Append(row.left.name)

	if prop.Description != "" {
		row.left.desc = gtk.NewLabel(prop.Description)
		row.left.desc.AddCSSClass("prefui-prop-description")
		row.left.desc.SetUseMarkup(true)
		row.left.desc.SetXAlign(0)
		row.left.desc.SetWrap(true)
		row.left.desc.SetWrapMode(pango.WrapWordChar)
		row.left.Append(row.left.desc)
	}

	row.action = newPropWidget(d, prop)
	gtk.BaseWidget(row.action).SetVAlign(gtk.AlignCenter)

	orientation := gtk.OrientationHorizontal
	if row.action.long {
		orientation = gtk.OrientationVertical
	}

	row.box = gtk.NewBox(orientation, 0)
	row.box.Append(row.left)
	row.box.Append(row.action)

	row.SetChild(row.box)

	return &row
}

func (r *propRow) Activate() bool {
	return gtk.BaseWidget(r.action).Activate()
}

type propWidget struct {
	gtk.Widgetter
	long bool
}

func newPropWidget(d *Dialog, p prefs.LocalizedProp) propWidget {
	switch p := p.Prop.(type) {
	case *prefs.Bool:
		// Note: using gtk.Switch actually causes a crash using the Cairo
		// renderer for some reason. The log goes:
		//
		//    2022/01/16 16:38:37 Message: Gsk: Failed to realize renderer of type 'GskNglRenderer' for surface 'GdkWaylandPopup': Failed to create EGL display
		//    2022/01/16 16:38:37 Message: Gsk: Failed to realize renderer of type 'GskNglRenderer' for surface 'GdkWaylandToplevel': Failed to create EGL display
		//    2022/01/16 16:38:37 Warning: Gsk: drawing failure for render node GskTextureNode: invalid matrix (not invertible)
		//    gotktrix: cairo-surface.c:2995: _cairo_surface_create_in_error: Assertion `status < CAIRO_STATUS_LAST_STATUS' failed.
		//    SIGABRT: abort
		//    PC=0x7fe2a496ebaa m=3 sigcode=18446744073709551610
		//
		// See issue https://gitlab.gnome.org/GNOME/gtk/-/issues/4642.
		sw := gtk.NewSwitch()
		sw.AddCSSClass("prefui-prop")
		sw.AddCSSClass("prefui-prop-bool")
		bindPropWidget(d, p, sw, "notify::active", propFuncs{
			set:     func() { sw.SetActive(p.Value()) },
			publish: func() { p.Publish(sw.Active()) },
		})
		return propWidget{
			Widgetter: sw,
			long:      false,
		}
	case *prefs.Int:
		min := float64(p.Min)
		max := float64(p.Max)
		if p.Slider {
			slider := gtk.NewScaleWithRange(gtk.OrientationHorizontal, min, max, 1)
			slider.AddCSSClass("prefui-prop")
			slider.AddCSSClass("prefui-prop-int")
			bindPropWidget(d, p, slider, "changed", propFuncs{
				set:     func() { slider.SetValue(float64(p.Value())) },
				publish: func() { p.Publish(roundInt(slider.Value())) },
			})
			return propWidget{
				Widgetter: slider,
				long:      true,
			}
		} else {
			spin := gtk.NewSpinButtonWithRange(min, max, 1)
			spin.AddCSSClass("prefui-prop")
			spin.AddCSSClass("prefui-prop-int")
			bindPropWidget(d, p, spin, "value-changed", propFuncs{
				set:     func() { spin.SetValue(float64(p.Value())) },
				publish: func() { p.Publish(spin.ValueAsInt()) },
			})
			return propWidget{
				Widgetter: spin,
				long:      false,
			}
		}
	case *prefs.String:
		// TODO: multiline
		entry := gtk.NewEntry()
		entry.AddCSSClass("prefui-prop")
		entry.AddCSSClass("prefui-prop-string")
		entry.SetWidthChars(10)
		entry.SetPlaceholderText(locale.S(d.ctx, p.Placeholder))
		entry.ConnectChanged(func() {
			setEntryIcon(entry, "object-select", "")
		})
		bindPropWidget(d, p, entry, "activate,icon-press", propFuncs{
			set: func() {
				entry.SetText(p.Value())
			},
			publish: func() bool {
				if err := p.Publish(entry.Text()); err != nil {
					setEntryIcon(entry, "dialog-error", "Error: "+err.Error())
					return false
				} else {
					setEntryIcon(entry, "object-select", "")
					return true
				}
			},
		})
		return propWidget{
			Widgetter: entry,
			long:      true,
		}
	default:
		log.Panicf("unknown property type %T", p)
		panic("")
	}
}

func setEntryIcon(entry *gtk.Entry, icon, text string) {
	entry.SetIconFromIconName(gtk.EntryIconSecondary, icon)
	entry.SetIconTooltipText(gtk.EntryIconSecondary, text)
}

func roundInt(v float64) int {
	return int(math.Round(v))
}

type propFuncs struct {
	set     func()
	publish interface{}
}

func bindPropWidget(d *Dialog, p prefs.Prop, w gtk.Widgetter, changed string, funcs propFuncs) {
	var paused bool

	activate := func() {
		if paused {
			return
		}

		switch publish := funcs.publish.(type) {
		case func():
			publish()
		case func() bool:
			if !publish() {
				return
			}
		case func() error:
			if err := publish(); err != nil {
				return
			}
		default:
			log.Panicf("unknown publish callback type %T", publish)
		}

		d.save()
	}

	for _, signal := range strings.Split(changed, ",") {
		w.Connect(signal, activate)
	}

	p.Pubsubber().SubscribeWidget(w, func() {
		paused = true
		funcs.set()
		paused = false
	})
}
