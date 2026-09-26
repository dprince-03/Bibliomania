package reading

import (
	"context"
	"errors"
	"time"

	apperrors "github.com/dprince-03/Bibliomania/internal/errors"
	"github.com/dprince-03/Bibliomania/internal/events"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Store is reading-service's persistence. Every write that emits an event
// takes the events and commits them with the write (MongoOutbox) in one
// multi-document transaction.
type Store interface {
	GetSession(ctx context.Context, userID, bookID uint64) (*ReadingSession, error)
	ListSessions(ctx context.Context, userID uint64, limit, offset int) ([]*ReadingSession, int, error)
	// UpsertProgress applies s with last-write-wins on ClientUpdatedAtNs.
	// eventsFor receives the stored state *before* the write (nil for a new
	// session) and whether the write won, and returns the events to commit.
	UpsertProgress(ctx context.Context, s *ReadingSession, eventsFor func(prev *ReadingSession, applied bool) ([]events.Event, error)) error

	ListBookmarks(ctx context.Context, userID, bookID uint64) ([]*Bookmark, error)
	GetBookmark(ctx context.Context, id uint64) (*Bookmark, error)
	CreateBookmark(ctx context.Context, b *Bookmark) error
	DeleteBookmark(ctx context.Context, id uint64) error

	Ping(ctx context.Context) error
}

type store struct {
	client    *mongo.Client
	sessions  *mongo.Collection
	bookmarks *mongo.Collection
	counters  *mongo.Collection
	outbox    *events.MongoOutbox
}

func NewStore(client *mongo.Client, db *mongo.Database, outbox *events.MongoOutbox) Store {
	return &store{
		client:    client,
		sessions:  db.Collection("reading_sessions"),
		bookmarks: db.Collection("bookmarks"),
		counters:  db.Collection("counters"),
		outbox:    outbox,
	}
}

// EnsureIndexes is reading-service's "migration": MongoDB is schemaless, so
// the only schema to manage is its indexes.
func EnsureIndexes(ctx context.Context, db *mongo.Database) error {
	if _, err := db.Collection("reading_sessions").Indexes().CreateMany(ctx, []mongo.IndexModel{
		// One session per user per book.
		{Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "book_id", Value: 1}}, Options: options.Index().SetUnique(true)},
		// History, newest first.
		{Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "last_read_at", Value: -1}}},
	}); err != nil {
		return err
	}
	_, err := db.Collection("bookmarks").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "book_id", Value: 1}, {Key: "page", Value: 1}},
	})
	return err
}

func (s *store) Ping(ctx context.Context) error {
	return s.client.Ping(ctx, nil)
}

func (s *store) GetSession(ctx context.Context, userID, bookID uint64) (*ReadingSession, error) {
	var session ReadingSession
	err := s.sessions.FindOne(ctx, bson.M{"user_id": userID, "book_id": bookID}).Decode(&session)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, apperrors.NotFound("reading session")
	}
	if err != nil {
		return nil, apperrors.Internal(err)
	}
	return &session, nil
}

func (s *store) ListSessions(ctx context.Context, userID uint64, limit, offset int) ([]*ReadingSession, int, error) {
	filter := bson.M{"user_id": userID}
	total, err := s.sessions.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, apperrors.Internal(err)
	}

	cur, err := s.sessions.Find(ctx, filter, options.Find().
		SetSort(bson.D{{Key: "last_read_at", Value: -1}}).
		SetSkip(int64(offset)).
		SetLimit(int64(limit)))
	if err != nil {
		return nil, 0, apperrors.Internal(err)
	}
	sessions := []*ReadingSession{}
	if err := cur.All(ctx, &sessions); err != nil {
		return nil, 0, apperrors.Internal(err)
	}
	return sessions, int(total), nil
}

// UpsertProgress implements last-write-wins atomically:
//
//  1. update the existing session only if its clock is strictly older
//     (`$lt` — on a tie the existing row wins, matching the pre-split SQL);
//  2. if nothing matched and a session exists, it has a newer-or-equal
//     clock: this write lost and is silently discarded;
//  3. otherwise insert a new session.
//
// Everything, including the outbox events, runs in one transaction. Step
// 2 is an explicit lookup rather than "insert and catch the duplicate-key
// error": inside a MongoDB transaction that error aborts the whole
// transaction. The one remaining duplicate-key case — two first writes
// for the same session racing — is retried once, and then takes path 1/2.
func (s *store) UpsertProgress(ctx context.Context, sess *ReadingSession, eventsFor func(prev *ReadingSession, applied bool) ([]events.Event, error)) error {
	err := s.upsertProgressTx(ctx, sess, eventsFor)
	if err != nil && mongo.IsDuplicateKeyError(err) {
		err = s.upsertProgressTx(ctx, sess, eventsFor)
	}
	return err
}

func (s *store) upsertProgressTx(ctx context.Context, sess *ReadingSession, eventsFor func(prev *ReadingSession, applied bool) ([]events.Event, error)) error {
	return s.inTx(ctx, func(ctx context.Context) error {
		now := time.Now().UTC()
		filter := bson.M{
			"user_id":              sess.UserID,
			"book_id":              sess.BookID,
			"client_updated_at_ns": bson.M{"$lt": sess.ClientUpdatedAtNs},
		}
		update := bson.M{"$set": bson.M{
			"current_page":         sess.CurrentPage,
			"total_pages":          sess.TotalPages,
			"progress_pct":         sess.ProgressPct,
			"current_chapter":      sess.CurrentChapter,
			"is_completed":         sess.IsCompleted,
			"completed_at":         sess.CompletedAt,
			"client_updated_at_ns": sess.ClientUpdatedAtNs,
			"last_read_at":         now,
			"updated_at":           now,
		}}

		var prev ReadingSession
		err := s.sessions.FindOneAndUpdate(ctx, filter, update,
			options.FindOneAndUpdate().SetReturnDocument(options.Before)).Decode(&prev)

		var prevPtr *ReadingSession
		applied := true
		switch {
		case err == nil:
			prevPtr = &prev
		case errors.Is(err, mongo.ErrNoDocuments):
			n, err := s.sessions.CountDocuments(ctx, bson.M{"user_id": sess.UserID, "book_id": sess.BookID})
			if err != nil {
				return err
			}
			if n > 0 {
				applied = false // an equal-or-newer write already won
				break
			}
			sess.StartedAt, sess.LastReadAt, sess.CreatedAt, sess.UpdatedAt = now, now, now, now
			if _, err := s.sessions.InsertOne(ctx, sess); err != nil {
				return err
			}
		default:
			return err
		}

		evs, err := eventsFor(prevPtr, applied)
		if err != nil {
			return err
		}
		for _, e := range evs {
			if err := s.outbox.Add(ctx, e); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *store) ListBookmarks(ctx context.Context, userID, bookID uint64) ([]*Bookmark, error) {
	cur, err := s.bookmarks.Find(ctx, bson.M{"user_id": userID, "book_id": bookID},
		options.Find().SetSort(bson.D{{Key: "page", Value: 1}}))
	if err != nil {
		return nil, apperrors.Internal(err)
	}
	bookmarks := []*Bookmark{}
	if err := cur.All(ctx, &bookmarks); err != nil {
		return nil, apperrors.Internal(err)
	}
	return bookmarks, nil
}

func (s *store) GetBookmark(ctx context.Context, id uint64) (*Bookmark, error) {
	var b Bookmark
	err := s.bookmarks.FindOne(ctx, bson.M{"_id": id}).Decode(&b)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, apperrors.NotFound("bookmark")
	}
	if err != nil {
		return nil, apperrors.Internal(err)
	}
	return &b, nil
}

func (s *store) CreateBookmark(ctx context.Context, b *Bookmark) error {
	var counter struct {
		Seq uint64 `bson:"seq"`
	}
	err := s.counters.FindOneAndUpdate(ctx,
		bson.M{"_id": "bookmarks"},
		bson.M{"$inc": bson.M{"seq": 1}},
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After),
	).Decode(&counter)
	if err != nil {
		return apperrors.Internal(err)
	}

	b.ID = counter.Seq
	b.CreatedAt = time.Now().UTC()
	if _, err := s.bookmarks.InsertOne(ctx, b); err != nil {
		return apperrors.Internal(err)
	}
	return nil
}

func (s *store) DeleteBookmark(ctx context.Context, id uint64) error {
	if _, err := s.bookmarks.DeleteOne(ctx, bson.M{"_id": id}); err != nil {
		return apperrors.Internal(err)
	}
	return nil
}

func (s *store) inTx(ctx context.Context, fn func(ctx context.Context) error) error {
	session, err := s.client.StartSession()
	if err != nil {
		return apperrors.Internal(err)
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(ctx context.Context) (any, error) {
		return nil, fn(ctx)
	})
	if err != nil {
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) {
			return err
		}
		return apperrors.Internal(err)
	}
	return nil
}
