package locale

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"path"
	"path/filepath"

	"github.com/pkg/errors"
	"golang.org/x/text/message/catalog"
	"golang.org/x/text/message/pipeline"
)

// LoadErrors is returned by LoadLocales when a language or more fails to be
// loaded.
type LoadErrors struct {
	Errors []struct {
		Language string
		Error    error
	}
}

// Error formats the LoadErrors strings.
func (errs *LoadErrors) Error() string {
	return fmt.Sprintf("cannot load %d languages: %q", len(errs.Errors), errs.Languages())
}

// Languages returns a list of languages that errored out.
func (errs *LoadErrors) Languages() []string {
	langs := make([]string, len(errs.Errors))
	for i, err := range errs.Errors {
		langs[i] = err.Language
	}
	return langs
}

func (errs *LoadErrors) add(lang string, err error) {
	errs.Errors = append(errs.Errors, struct {
		Language string
		Error    error
	}{lang, err})
}

// MustLoadLocales always returns a valid catalog. All errors are logged, not
// panicked.
func MustLoadLocales(locales fs.ReadDirFS) catalog.Catalog {
	l, err := LoadLocales(locales)
	if err != nil {
		var errs *LoadErrors
		if errors.As(err, &errs) {
			for _, err := range errs.Errors {
				log.Printf("locale: cannot load language %q: %v", err.Language, err.Error)
			}
		} else {
			log.Println("locale: cannot load:", err)
		}
	}
	return l
}

// LoadLocales loads all locales in the given filesystem into the global message
// catalog. All subfolders must be valid language tags.
func LoadLocales(locales fs.ReadDirFS) (catalog.Catalog, error) {
	parser := newLocaleParser(locales)
	lerror := LoadErrors{}

	ents, err := locales.ReadDir(".")
	if err != nil {
		return parser.bld, errors.Wrap(err, "cannot read locales dir")
	}

	for _, ent := range ents {
		if !ent.IsDir() {
			continue
		}

		files, err := locales.ReadDir(ent.Name())
		if err == nil {
			continue
		}

		for _, file := range files {
			if path.Ext(file.Name()) != ".json" {
				continue
			}

			l, err := parser.load(path.Join(ent.Name(), file.Name()))
			if err != nil {
				lerror.add(l, err)
			}
		}
	}

	if len(lerror.Errors) > 0 {
		return parser.bld, &lerror
	}

	return parser.bld, nil
}

type localeParser struct {
	bld *catalog.Builder
	fs  fs.ReadDirFS
}

func newLocaleParser(fs fs.ReadDirFS) localeParser {
	return localeParser{
		bld: catalog.NewBuilder(),
		fs:  fs,
	}
}

func (p localeParser) load(path string) (lang string, err error) {
	f, err := p.fs.Open(path)
	if err != nil {
		// TODO: don't use filepath.
		return pathDir(path), err
	}
	defer f.Close()

	var messages pipeline.Messages
	if err := json.NewDecoder(f).Decode(&messages); err != nil {
		return pathDir(path), errors.Wrap(err, "invalid catalog JSON")
	}

	lang = messages.Language.String()

	for _, message := range messages.Messages {
		// TODO: add more from message.
		trans, err := message.Substitute(message.Translation.Msg)
		if err != nil {
			return lang, errors.Wrapf(err,
				"cannot substitute translation message %q", message.Translation.Msg)
		}

		for _, id := range message.ID {
			if err := p.bld.SetString(messages.Language, id, trans); err != nil {
				return lang, errors.Wrapf(err,
					"%s: cannot set message ID %q", messages.Language, id)
			}
		}
	}

	return lang, nil
}

func pathDir(fullPath string) string {
	// TODO: don't use filepath.
	return filepath.Dir(fullPath)
}
