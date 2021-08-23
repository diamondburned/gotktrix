package indexer

import (
	"log"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/pkg/errors"
)

// QuerySize is the default size of each query.
const QuerySize = 25

// Indexer provides indexing of many types of Matrix data for querying.
type Indexer struct {
	idx bleve.Index
}

func Open(path string) (*Indexer, error) {
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
func (b BatchIndexer) IndexRoomMember(m event.RoomMemberEvent) {
	data := indexRoomMember(&m)
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
	qry *query.FuzzyQuery
}

// SearchRoomMember returns a new instance of RoomMemberSearcher that the client
// can use to search room members.
func (idx *Indexer) SearchRoomMember(roomID matrix.RoomID) RoomMemberSearcher {
	return RoomMemberSearcher{
		idx:  idx.idx,
		room: roomID,
		size: QuerySize,
	}
}

// Search looks up the indexing database and searches for the given string. The
// returned list of IDs is valid until the next time Search is called.
func (s *RoomMemberSearcher) Search(str string) []IndexedRoomMember {
	if s.qry != nil {
		s.qry.Term = str
	} else {
		s.qry = query.NewFuzzyQuery(str)
		s.qry.FieldVal = "name"

		// Create an AND match so that only queries matching the RoomID is
		// searched on.
		and := query.NewConjunctionQuery([]query.Query{
			&query.MatchQuery{
				Match:    string(s.room),
				Prefix:   len(s.room),
				FieldVal: "room_id",
			},
			s.qry,
		})

		s.req = bleve.NewSearchRequestOptions(and, s.size, 0, false)
		s.req.SortBy([]string{"_score"})
		s.req.Fields = []string{"id", "room_id", "name"}
	}

	results, err := s.idx.Search(s.req)
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
