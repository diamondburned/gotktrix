// Package pronouns provides an implementation of the pronouns personal
// protocol.
package pronouns

import (
	"encoding/json"
	"strings"

	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotrix/event"
	"github.com/diamondburned/gotrix/matrix"
	"github.com/pkg/errors"
)

func init() {
	event.RegisterDefault(SelfPronounsEventType, parseSelfPronounsEvent)
	event.RegisterDefault(UserPronounsEventType, parseUserPronounsEvent)
}

const (
	SelfPronounsEventType event.Type = "xyz.diamondb.gotktrix.self_pronouns"
	UserPronounsEventType event.Type = "xyz.diamondb.gotktrix.user_pronouns"
)

// SelfPronounsEvent describes the xyz.diamondb.gotktrix.self_pronouns event. It
// is an event that others
type SelfPronounsEvent struct {
	event.EventInfo `json:"-"`

	Self   Preferred                   `json:"self"`
	Others map[matrix.UserID]Preferred `json:"others,omitempty"`
}

func parseSelfPronounsEvent(content json.RawMessage) (event.Event, error) {
	var ev SelfPronounsEvent
	err := json.Unmarshal(content, &ev)
	return &ev, err
}

// UserPronounsEvent describes the xyz.diamondb.gotktrix.user_pronouns event.
// This event is propagated to rooms as a state event for a user.
type UserPronounsEvent struct {
	event.StateEventInfo `json:"-"`

	UserID matrix.UserID `json:"user_id"`
	Preferred
}

func parseUserPronounsEvent(content json.RawMessage) (event.Event, error) {
	var ev UserPronounsEvent
	err := json.Unmarshal(content, &ev)
	return &ev, err
}

// Preferred lists preferred pronouns.
type Preferred struct {
	Pronouns []Pronoun `json:"pronouns"`
	// Preferred is indexed within Pronouns; the default is the first entry.
	Preferred int `json:"preferred,omitempty"`
}

// Pronoun returns the preferred pronoun.
func (p Preferred) Pronoun() Pronoun {
	if len(p.Pronouns) == 0 {
		return ""
	}

	if p.Preferred < 0 || p.Preferred >= len(p.Pronouns) {
		return p.Pronouns[0]
	}

	return p.Pronouns[p.Preferred]
}

// Pronoun describes a pronoun string in the format they/them/theirs.
type Pronoun string

// PronounForms describes the 3 forms of pronouns.
type PronounForms struct {
	Subject    string
	Object     string
	Possessive string
}

// DefaultPronouns is the list of default common pronouns. A client should not
// assume that valid pronoun choices can only be one of these three.
var DefaultPronouns = []PronounForms{
	{"he", "him", "his"},
	{"she", "her", "hers"},
	{"they", "them", "theirs"},
}

// ParsePronoun parses the given pronoun in X/Y/Z form, similarly to what
// String() returns. Spaces inbetween pronouns will be trimmed.
//
// Examples of valid pronoun strings are:
//
//    he/him/his
//    he / him / his
//
//    she/her/hers
//    she    / her      / hers
//
//    they  /     them / theirs
//    they/them/theirs
//
func ParsePronoun(pronoun Pronoun) (PronounForms, error) {
	parts := strings.Split(string(pronoun), "/")
	switch len(parts) {
	case 0:
		return PronounForms{}, errors.New("no pronoun given")
	case 1:
		return PronounForms{parts[0], "", ""}, nil
	case 2:
		return PronounForms{parts[0], parts[1], ""}, nil
	case 3:
		return PronounForms{parts[0], parts[1], parts[2]}, nil
	default:
		return PronounForms{}, errors.New("too many forms given")
	}
}

// String returns the pronoun in X/Y/Z form. If PronounForms is a zero-value,
// then an empty string is returned.
func (p PronounForms) String() string {
	switch {
	case p.Subject == "":
		return ""
	case p.Object == "":
		return p.Subject
	case p.Possessive == "":
		return p.Subject + "/" + p.Object
	default:
		return p.Subject + "/" + p.Object + "/" + p.Possessive
	}
}

// SubjectObject is similar to String, except the Possessive form is omitted.
func (p PronounForms) SubjectObject() string {
	switch {
	case p.Subject == "":
		return ""
	case p.Object == "":
		return p.Subject
	default:
		return p.Subject + "/" + p.Object
	}
}

// SelfPronouns fetches the current user's preferred pronouns. A zero-value
// Preferred instance is returned if the user has none.
func SelfPronouns(c *gotktrix.Client) Preferred {
	e, err := c.State.UserEvent(UserPronounsEventType)
	if err != nil {
		return Preferred{}
	}

	ev, _ := e.(*SelfPronounsEvent)
	if ev != nil {
		return ev.Self
	}

	return Preferred{}
}

// UserPronouns searches the room for the given user's preferred pronouns. If
// the room ID is empty, then the user ID is only searched in the current user
// preferences, or if the user ID belongs to the current user, then their own
// preference is returned (i.e. it acts like SelfPronouns).
func UserPronouns(c *gotktrix.Client, rID matrix.RoomID, uID matrix.UserID) Preferred {
	if rID != "" {
		if e, _ := c.State.RoomState(rID, UserPronounsEventType, string(uID)); e != nil {
			return e.(*UserPronounsEvent).Preferred
		}
	}

	// Query the user's preference instead.
	if e, _ := c.State.UserEvent(SelfPronounsEventType); e != nil {
		pronouns := e.(*SelfPronounsEvent)

		p, ok := pronouns.Others[uID]
		if ok {
			return p
		}

		// Check if the given user ID is the current user.
		u, err := c.Whoami()
		if err == nil && u == uID {
			return pronouns.Self
		}
	}

	return Preferred{}
}
