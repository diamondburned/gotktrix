// Package pronouns provides an implementation of the pronouns personal
// protocol.
package pronouns

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/pkg/errors"
)

func init() {
	event.Register(SelfPronounsEventType, parseSelfPronounsEvent)
	event.Register(UserPronounsEventType, parseUserPronounsEvent)
}

const (
	SelfPronounsEventType event.Type = "xyz.diamondb.gotktrix.self_pronouns"
	UserPronounsEventType event.Type = "xyz.diamondb.gotktrix.user_pronouns"
)

// SelfPronounsEvent describes the xyz.diamondb.gotktrix.self_pronouns event. It
// is an event that others
type SelfPronounsEvent struct {
	Self   Preferred                   `json:"self"`
	Others map[matrix.UserID]Preferred `json:"others,omitempty"`
}

func parseSelfPronounsEvent(raw event.RawEvent) (event.Event, error) {
	var ev SelfPronounsEvent
	if raw.Type != ev.Type() {
		return nil, fmt.Errorf("unexpected event type %q for SelfPronounsEvent", raw.Type)
	}

	if err := json.Unmarshal(raw.Content, &ev); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal SelfPronounsEvent")
	}

	return ev, nil
}

// Type implements event.Type.
func (ev SelfPronounsEvent) Type() event.Type { return SelfPronounsEventType }

// UserPronounsEvent describes the xyz.diamondb.gotktrix.user_pronouns event.
// This event is propagated to rooms as a state event for a user.
type UserPronounsEvent struct {
	event.RoomEventInfo
	UserID matrix.UserID `json:"user_id"`

	Preferred
}

func parseUserPronounsEvent(raw event.RawEvent) (event.Event, error) {
	var ev UserPronounsEvent
	if raw.Type != ev.Type() {
		return nil, fmt.Errorf("unexpected event type %q for UserPronounsEvent", raw.Type)
	}

	if err := json.Unmarshal(raw.Content, &ev); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal UserPronounsEvent")
	}

	return ev, nil
}

// Type implements event.Type.
func (ev UserPronounsEvent) Type() event.Type { return UserPronounsEventType }

// StateKey returns the user ID.
func (ev UserPronounsEvent) StateKey() string { return string(ev.UserID) }

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

// SelfPronouns fetches the current user's preferred pronouns. A zero-value
// Preferred instance is returned if the user has none.
func SelfPronouns(c *gotktrix.Client) Preferred {
	e, err := c.UserEvent(UserPronounsEventType)
	if err != nil {
		return Preferred{}
	}

	return e.(SelfPronounsEvent).Self
}

// UserPronouns searches the room for the given user's preferred pronouns. If
// the room ID is empty, then the user ID is only searched in the current user
// preferences, or if the user ID belongs to the current user, then their own
// preference is returned (i.e. it acts like SelfPronouns).
func UserPronouns(c *gotktrix.Client, rID matrix.RoomID, uID matrix.UserID) Preferred {
	if rID != "" {
		if e, _ := c.RoomState(rID, UserPronounsEventType, string(uID)); e != nil {
			return e.(UserPronounsEvent).Preferred
		}
	}

	// Query the user's preference instead.
	if e, _ := c.UserEvent(SelfPronounsEventType); e != nil {
		pronouns := e.(SelfPronounsEvent)

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
