package text

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil/textutil"
)

var allowedSchemes = map[string]struct{}{
	"http":   {},
	"https":  {},
	"ftp":    {},
	"ftps":   {},
	"mailto": {},
	"magnet": {},
}

func urlIsSafe(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}

	_, safe := allowedSchemes[u.Scheme]
	return safe
}

const embeddedURLPrefix = "link:"

// Regex written by @stephenhay, taken from https://mathiasbynens.be/demo/url-regex.
// Mirror this to allowedSchemes.
var urlRegex = regexp.MustCompile(`(?:https?|ftps?|mailto|magnet)://[^\s/$.?#].[^\s]*`)

// hyperlink turns all URLs inside the given text to be wrapped around <a> tags.
// The text is not HTML-escaped.
func hyperlink(html string) (string, []string) {
	var urls []string
	html = urlRegex.ReplaceAllStringFunc(html, func(url string) string {
		urls = append(urls, url)
		return fmt.Sprintf(`<a href="%s">%[1]s</a>`, url)
	})
	return html, urls
}

// autolink scans the buffer's text for all unhighlighted URLs and highlight
// them. Tags that are autolinked will pass the IsEmbeddedURLTag check.
func autolink(buf *gtk.TextBuffer) []string {
	table := buf.TagTable()

	start, end := buf.Bounds()
	text := buf.Slice(start, end, true)

	var urls []string

matchLoop:
	for _, match := range urlRegex.FindAllStringIndex(text, -1) {
		// match[0] : match[1]
		urls = append(urls, text[match[0]:match[1]])

		// Count lines.
		line := strings.Count(text[:match[0]], "\n")

		// Get the offset, in bytes, relative to text, of this line.
		lineAt := 0
		if line > 0 {
			lineAt = strings.LastIndexByte(text[:match[0]], '\n') + 1
		}

		// Count the tuple's offset using the new line index.
		offset0 := match[0] - lineAt
		offset1 := match[1] - lineAt

		start.SetLine(line)
		start.SetLineIndex(offset0)

		// Ensure that the start iterator doesn't already have a link tag, nor
		// is it invisible. Skip if it does/is.
		for _, tag := range start.Tags() {
			tagName := tag.ObjectProperty("name").(string)
			if tagName == "_invisible" || strings.HasPrefix(tagName, embeddedURLPrefix) {
				continue matchLoop
			}
		}

		end.SetLine(line)
		end.SetLineIndex(offset1)

		a := textutil.LinkTags().FromTable(table, "a")
		buf.ApplyTag(a, start, end)

		href := text[match[0]:match[1]]
		link := emptyTag(table, embeddedURLPrefix+embedURL(start.Offset(), end.Offset(), href))
		buf.ApplyTag(link, start, end)
	}

	return urls
}

// BindLinkHandler binds input handlers for triggering hyperlinks within the
// TextView.
func BindLinkHandler(tview *gtk.TextView, onURL func(string)) {
	linkTags := textutil.LinkTags()

	checkURL := func(x, y float64) *EmbeddedURL {
		bx, by := tview.WindowToBufferCoords(gtk.TextWindowWidget, int(x), int(y))
		it, ok := tview.IterAtLocation(bx, by)
		if !ok {
			return nil
		}

		for _, tags := range it.Tags() {
			tagName := tags.ObjectProperty("name").(string)

			if !strings.HasPrefix(tagName, embeddedURLPrefix) {
				continue
			}

			u, ok := ParseEmbeddedURL(strings.TrimPrefix(tagName, embeddedURLPrefix))
			if ok {
				return &u
			}
		}

		return nil
	}

	var buf *gtk.TextBuffer
	var table *gtk.TextTagTable
	var iters [2]*gtk.TextIter

	needIters := func() {
		if buf == nil {
			buf = tview.Buffer()
			table = buf.TagTable()
		}

		if iters == [2]*gtk.TextIter{} {
			i1 := buf.IterAtOffset(0)
			i2 := buf.IterAtOffset(0)
			iters = [2]*gtk.TextIter{i1, i2}
		}
	}

	click := gtk.NewGestureClick()
	click.SetButton(1)
	click.SetExclusive(true)
	click.ConnectAfter("pressed", func(nPress int, x, y float64) {
		if nPress != 1 {
			return
		}

		if u := checkURL(x, y); u != nil {
			onURL(u.URL)

			needIters()
			tag := linkTags.FromBuffer(buf, "a:visited")

			iters[0].SetOffset(u.From)
			iters[1].SetOffset(u.To)

			buf.ApplyTag(tag, iters[0], iters[1])
		}
	})

	var (
		lastURL *EmbeddedURL
		lastTag *gtk.TextTag
	)

	unhover := func() {
		if lastURL != nil {
			needIters()
			iters[0].SetOffset(lastURL.From)
			iters[1].SetOffset(lastURL.To)
			buf.RemoveTag(lastTag, iters[0], iters[1])

			lastURL = nil
			lastTag = nil
		}
	}

	motion := gtk.NewEventControllerMotion()
	motion.ConnectLeave(func() {
		unhover()
		iters = [2]*gtk.TextIter{}
	})
	motion.ConnectMotion(func(x, y float64) {
		u := checkURL(x, y)
		if u == lastURL {
			return
		}

		unhover()

		if u != nil {
			needIters()
			iters[0].SetOffset(u.From)
			iters[1].SetOffset(u.To)

			hover := linkTags.FromTable(table, "a:hover")
			buf.ApplyTag(hover, iters[0], iters[1])

			lastURL = u
			lastTag = hover
		}
	})

	tview.AddController(click)
	tview.AddController(motion)
}

// EmbeddedURL is a type that describes a URL and its bounds within a text
// buffer.
type EmbeddedURL struct {
	From int    `json:"1"`
	To   int    `json:"2"`
	URL  string `json:"u"`
}

func embedURL(x, y int, url string) string {
	b, err := json.Marshal(EmbeddedURL{x, y, url})
	if err != nil {
		log.Panicln("bug: error marshaling embeddedURL:", err)
	}

	return string(b)
}

// ParseEmbeddedURL parses the inlined data into an embedded URL structure.
func ParseEmbeddedURL(data string) (EmbeddedURL, bool) {
	var d EmbeddedURL
	err := json.Unmarshal([]byte(data), &d)
	return d, err == nil
}

// Bounds returns the bound iterators from the given text buffer.
func (e *EmbeddedURL) Bounds(buf *gtk.TextBuffer) (start, end *gtk.TextIter) {
	startIter := buf.IterAtOffset(e.From)
	endIter := buf.IterAtOffset(e.To)
	return startIter, endIter
}
