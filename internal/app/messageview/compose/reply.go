package compose

import (
	"html/template"
	"log"
	"strings"
	"unicode/utf8"

	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotrix/event"
	"github.com/diamondburned/gotrix/matrix"
)

// overkill lol

type replyData struct {
	RoomID     matrix.RoomID
	EventID    matrix.EventID
	SenderID   matrix.UserID
	SenderName string
	Content    string
}

const replyHTML = `
	<mx-reply>
		<blockquote>
			<a href="https://matrix.to/#/{{.RoomID}}/{{.EventID}}">In reply to</a> 
			<a href="https://matrix.to/#/{{.SenderID}}">{{.SenderName}}</a>
			<br>{{ .Content }}
		</blockquote>
	</mx-reply>
`

var templateFormatter = strings.NewReplacer("\n", "", "\t", "")

var replyTemplate = template.Must(
	template.New("reply").Parse(
		// Collapse all new lines, because we're relying on <br> instead.
		templateFormatter.Replace(replyHTML),
	),
)

func renderReply(
	html, plain *strings.Builder, client *gotktrix.Client, msg *event.RoomMessageEvent) {

	var name string
	if n, err := client.MemberName(msg.RoomID, msg.Sender, false); err == nil {
		name = n.Name
	} else {
		name, _, _ = msg.Sender.Parse()
	}

	data := replyData{
		RoomID:     msg.RoomID,
		EventID:    msg.ID,
		SenderID:   msg.Sender,
		SenderName: name,
		Content:    trim(msg.Body, 128),
	}

	plain.WriteString("> ")
	plain.WriteString(data.SenderName)
	plain.WriteString(": ")
	plain.WriteString(data.Content)
	plain.WriteString("\n")

	if err := replyTemplate.Execute(html, data); err != nil {
		log.Panicln("compose: failed to render reply HTML:", err)
	}
}

var spaceReplacer = strings.NewReplacer(
	"\n", " ", "\t", " ",
)

func trim(str string, max int) string {
	str = spaceReplacer.Replace(str)
	str = strings.TrimSpace(str)

	var len int
	for {
		_, sz := utf8.DecodeRuneInString(str[len:])
		if sz == 0 {
			break
		}

		if len += sz; len > max {
			return str[:len] + "…"
		}
	}

	return str
}
