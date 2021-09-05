package compose

import (
	"html/template"
	"log"
	"strings"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
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

var spaceReplacer = strings.NewReplacer(
	"\n", " ", "\t", " ",
)

var replyTemplate = template.Must(
	template.New("reply").Parse(
		// Collapse all new lines, because we're relying on <br> instead.
		spaceReplacer.Replace(replyHTML),
	),
)

func renderReply(out *strings.Builder, client *gotktrix.Client, msg *event.RoomMessageEvent) {
	var name string
	if n, err := client.MemberName(msg.RoomID, msg.SenderID, false); err == nil {
		name = n.Name
	} else {
		name, _, _ = msg.SenderID.Parse()
	}

	data := replyData{
		RoomID:     msg.RoomID,
		EventID:    msg.EventID,
		SenderID:   msg.SenderID,
		SenderName: name,
		Content:    trim(msg.Body, 128),
	}

	if err := replyTemplate.Execute(out, data); err != nil {
		log.Panicln("compose: failed to render reply HTML:", err)
	}
}

func trim(str string, max int) string {
	str = spaceReplacer.Replace(str)
	str = strings.TrimSpace(str)

	if len(str) > max {
		return str[:max] + "…"
	}
	return str
}
