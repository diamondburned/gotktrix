// Package pronouns provides an implementation of the pronouns personal
// protocol.
package pronouns

import (
	"errors"
	"fmt"
	"strings"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
)

const (
	SelfPronounsEventType event.Type = "xyz.diamondb.gotktrix.self_pronouns"
	UserPronounsEventType event.Type = "xyz.diamondb.gotktrix.user_pronouns"
)

// SelfPronounsEvent describes the xyz.diamondb.gotktrix.self_pronouns event. It
// is an event that others
type SelfPronounsEvent struct {
	Self   Preferred                   `json:"self"`
	Others map[matrix.UserID]Preferred `json:"others"`
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

// Type implements event.Type.
func (ev UserPronounsEvent) Type() event.Type { return UserPronounsEventType }

// StateKey returns the user ID.
func (ev UserPronounsEvent) StateKey() string { return string(ev.UserID) }

// Preferred lists preferred pronouns.
type Preferred struct {
	Pronouns  []string `json:"pronouns"`  // format: they/them/theirs, etc.
	Preferred int      `json:"preferred"` // index within Pronouns
}

// Pronoun describes the 3 grammatical forms of the pronoun.
type Pronoun struct {
	Subject    string
	Object     string
	Possessive string
}

// DefaultPronouns is the list of default common pronouns. A client should not
// assume that valid pronoun choices can only be one of these three.
var DefaultPronouns = []Pronoun{
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
func ParsePronoun(pronoun string) (Pronoun, error) {
	parts := strings.Split(pronoun, "/")
	switch len(parts) {
	case 0:
		return Pronoun{}, errors.New("no pronoun given")
	case 1:
		return Pronoun{}, errors.New("missing object grammatical form")
	case 2:
		return Pronoun{}, errors.New("missing possessive grammatical form")
	case 3:
		break // ok
	case 4:
		return Pronoun{}, errors.New("too many forms given")
	}

	return Pronoun{
		Subject:    strings.TrimSpace(parts[0]),
		Object:     strings.TrimSpace(parts[1]),
		Possessive: strings.TrimSpace(parts[2]),
	}, nil
}

// String returns the pronoun in X/Y/Z form.
func (p Pronoun) String() string {
	return fmt.Sprintf("%s/%s/%s", p.Subject, p.Object, p.Possessive)
}
