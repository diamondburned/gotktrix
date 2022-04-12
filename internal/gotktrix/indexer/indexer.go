package indexer

import (
	"context"
	"log"
	"sync"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/diamondburned/gotrix/event"
	"github.com/diamondburned/gotrix/matrix"
	"github.com/pkg/errors"
)

var initOnce sync.Once

func doInit() {
	// The fact that bleve.Config is a global variable of an unexported type is
	// kind of stupid.
	initOnce.Do(func() {
		// Set the number of background Bleve indexers to 1, because we don't do
		// that much searching.
		bleve.Config.SetAnalysisQueueSize(1)
		// Not too sure what this does, but we don't need it to be set to
		// "html".
		bleve.Config.DefaultHighlighter = "simple"
	})
}

// Indexer provides indexing of many types of Matrix data for querying.
type Indexer struct {
	idx bleve.Index
}

// Open opens an existing Indexer or create a new one if not available.
func Open(path string) (*Indexer, error) {
	doInit()

	// Work around Bleve's inherent TOCTTOU racy API.
	var idx bleve.Index
	// TODO: index database versioning
	for {
		x, err := bleve.Open(path)
		if err == nil {
			idx = x
			break
		}

		x, err = bleve.New(path, bleve.NewIndexMapping())
		if err == nil {
			idx = x
			break
		}

		if errors.Is(err, bleve.ErrorIndexPathExists) {
			// We can retry this loop.
			continue
		}

		return nil, errors.Wrap(err, "failed to initialize bleve")
	}

	return &Indexer{idx}, nil
}

// BatchIndexer wraps around a Bleve indexer for batch writing.
type BatchIndexer struct {
	idx bleve.Index
	b   *bleve.Batch
}

// Begin creates a new batch indexer.
func (idx *Indexer) Begin() BatchIndexer {
	return BatchIndexer{
		idx: idx.idx,
		b:   idx.idx.NewBatch(),
	}
}

// Commit commits the batched writes.
func (b BatchIndexer) Commit() {
	if err := b.idx.Batch(b.b); err != nil {
		log.Println("indexer error: while commiting:", err)
	}
}

// IndexRoomMember indexes the given room member from the event.
func (b BatchIndexer) IndexRoomMember(m *event.RoomMemberEvent) {
	data := indexRoomMember(m)
	b.index(&data)
}

type indexable interface {
	Index(*bleve.Batch) error
}

func (b BatchIndexer) index(indexer indexable) {
	if err := indexer.Index(b.b); err != nil {
		log.Println("indexer error:", err)
	}
}

type RoomMemberSearcher struct {
	// constants
	room matrix.RoomID
	idx  bleve.Index
	size int

	// state
	res []IndexedRoomMember
	req *bleve.SearchRequest

	queries []query.Query
}

const searchLimit = 25

// SearchRoomMember returns a new instance of RoomMemberSearcher that the client
// can use to search room members.
func (idx *Indexer) SearchRoomMember(roomID matrix.RoomID, limit int) RoomMemberSearcher {
	return RoomMemberSearcher{
		idx:  idx.idx,
		room: roomID,
		size: limit,
		res:  make([]IndexedRoomMember, 0, searchLimit),
	}
}

// Search looks up the indexing database and searches for the given string. The
// returned list of IDs is valid until the next time Search is called.
func (s *RoomMemberSearcher) Search(ctx context.Context, str string) []IndexedRoomMember {
	if s.queries != nil {
		// Set all known queries.
		for _, qry := range s.queries {
			switch qry := qry.(type) {
			case *query.FuzzyQuery:
				qry.Term = str
			case *query.TermQuery:
				qry.Term = str
			case *query.PrefixQuery:
				qry.Prefix = str
			default:
				log.Panicf("unknown query type %T", qry)
			}
		}
	} else {
		s.queries = []query.Query{
			&query.FuzzyQuery{Term: str, FieldVal: "name", Fuzziness: 1},
			&query.PrefixQuery{Prefix: str, FieldVal: "name"},
		}

		// Create an AND match so that only queries matching the RoomID is
		// searched on. It is written as (roomID AND (id OR name)).
		and := query.NewConjunctionQuery([]query.Query{
			&query.MatchQuery{
				Match:    string(s.room),
				Prefix:   len(s.room),
				FieldVal: "room_id",
			},
			// id OR name
			query.NewDisjunctionQuery(s.queries),
		})

		s.req = bleve.NewSearchRequestOptions(and, s.size, 0, false)
		s.req.Size = searchLimit
		s.req.Fields = []string{"id", "room_id", "name"}
		s.req.SortByCustom(search.SortOrder{
			// Highest-scored results first.
			&search.SortScore{Desc: true},
		})
	}

	results, err := s.idx.SearchInContext(ctx, s.req)
	if err != nil {
		log.Println("indexer: query error:", err)
		return nil
	}

	s.res = s.res[:0]
	for _, res := range results.Hits {
		s.res = append(s.res, IndexedRoomMember{
			ID:   matrix.UserID(res.Fields["id"].(string)),
			Room: matrix.RoomID(res.Fields["room_id"].(string)),
			Name: res.Fields["name"].(string),
		})
	}

	return s.res
}
