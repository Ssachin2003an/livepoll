package store

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"livepoll/backend/internal/model"
)

type Store struct {
	Client                        *mongo.Client
	DB                            *mongo.Database
	Users, Polls, Votes, Sessions *mongo.Collection
}

func Connect(ctx context.Context, uri, db string) (*Store, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri).SetWriteConcern(writeconcern.Majority()))
	if err != nil {
		return nil, err
	}
	if err = client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, err
	}
	store := &Store{Client: client, DB: client.Database(db)}
	store.Users = store.DB.Collection("users")
	store.Polls = store.DB.Collection("polls")
	store.Votes = store.DB.Collection("votes")
	store.Sessions = store.DB.Collection("sessions")
	indexes := []struct {
		collection *mongo.Collection
		model      mongo.IndexModel
	}{
		{store.Users, mongo.IndexModel{Keys: bson.D{{Key: "email", Value: 1}}, Options: options.Index().SetUnique(true).SetName("unique_normalized_email")}},
		{store.Polls, mongo.IndexModel{Keys: bson.D{{Key: "ownerId", Value: 1}, {Key: "createdAt", Value: -1}}, Options: options.Index().SetName("owner_created")}},
		{store.Votes, mongo.IndexModel{Keys: bson.D{{Key: "pollId", Value: 1}, {Key: "voterId", Value: 1}}, Options: options.Index().SetUnique(true).SetName("unique_poll_voter")}},
		{store.Votes, mongo.IndexModel{Keys: bson.D{{Key: "pollId", Value: 1}, {Key: "optionId", Value: 1}}, Options: options.Index().SetName("poll_option_results")}},
		{store.Votes, mongo.IndexModel{Keys: bson.D{{Key: "redisSynced", Value: 1}, {Key: "createdAt", Value: 1}}, Options: options.Index().SetName("pending_redis_sync")}},
		{store.Sessions, mongo.IndexModel{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0).SetName("session_expiry")}},
	}
	for _, index := range indexes {
		if _, err = index.collection.Indexes().CreateOne(ctx, index.model); err != nil {
			_ = client.Disconnect(context.Background())
			return nil, err
		}
	}
	return store, nil
}

func (s *Store) Close(ctx context.Context) error { return s.Client.Disconnect(ctx) }
func (s *Store) GetPoll(ctx context.Context, id string) (model.Poll, error) {
	var poll model.Poll
	err := s.Polls.FindOne(ctx, bson.M{"_id": id}).Decode(&poll)
	return poll, err
}
func (s *Store) Count(ctx context.Context, id string) (int64, error) {
	return s.Votes.CountDocuments(ctx, bson.M{"pollId": id})
}


func (s *Store) Result(ctx context.Context, poll model.Poll) (model.Results, error) {
	result := emptyResult(poll)

	cursor, err := s.Votes.Aggregate(ctx, mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.M{"pollId": poll.ID}}},
		bson.D{{Key: "$project", Value: bson.M{
			"selectedOptionIDs": bson.M{
				"$cond": bson.A{
					bson.M{
						"$gt": bson.A{
							bson.M{
								"$size": bson.M{
									"$ifNull": bson.A{"$optionIds", bson.A{}},
								},
							},
							0,
						},
					},
					"$optionIds",
					bson.A{"$optionId"},
				},
			},
		}}},
		bson.D{{Key: "$unwind", Value: "$selectedOptionIDs"}},
		bson.D{{Key: "$group", Value: bson.M{
			"_id": "$selectedOptionIDs",
			"n":   bson.M{"$sum": 1},
		}}},
	})
	if err != nil {
		return result, err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var row struct {
			ID string `bson:"_id"`
			N  int64  `bson:"n"`
		}
		if err = cursor.Decode(&row); err != nil {
			return result, err
		}
		if _, exists := result.Counts[row.ID]; exists {
			result.Counts[row.ID] = row.N
		}
	}
	if err = cursor.Err(); err != nil {
		return result, err
	}

	// Count vote records, not individual option selections.
	result.TotalVotes, err = s.Count(ctx, poll.ID)
	if err != nil {
		return result, err
	}

	calculatePercentages(&result, poll)
	return result, nil
}


// RebuildSnapshot derives both the durable counts and vote IDs from one Mongo cursor. Redis uses
// those IDs to prevent a pending outbox retry from counting a vote already included in the rebuild.

type VoteSnapshot struct {
	ID        string
	OptionIDs []string
}

func (s *Store) RebuildSnapshot(
	ctx context.Context,
	poll model.Poll,
) (model.Results, []VoteSnapshot, error) {
	result := emptyResult(poll)

	cursor, err := s.Votes.Find(
		ctx,
		bson.M{"pollId": poll.ID},
		options.Find().SetProjection(bson.M{
			"_id":       1,
			"optionId":  1,
			"optionIds": 1,
		}),
	)
	if err != nil {
		return result, nil, err
	}
	defer cursor.Close(ctx)

	snapshots := make([]VoteSnapshot, 0)

	for cursor.Next(ctx) {
		var row struct {
			ID        string   `bson:"_id"`
			OptionID  string   `bson:"optionId"`
			OptionIDs []string `bson:"optionIds"`
		}
		if err := cursor.Decode(&row); err != nil {
			return result, nil, err
		}

		selected := row.OptionIDs
		if len(selected) == 0 && row.OptionID != "" {
			selected = []string{row.OptionID}
		}

		snapshots = append(snapshots, VoteSnapshot{
			ID:        row.ID,
			OptionIDs: selected,
		})

		for _, optionID := range selected {
			if _, exists := result.Counts[optionID]; exists {
				result.Counts[optionID]++
			}
		}

		result.TotalVotes++
	}

	if err := cursor.Err(); err != nil {
		return result, nil, err
	}

	calculatePercentages(&result, poll)
	return result, snapshots, nil
}

func emptyResult(poll model.Poll) model.Results {
	result := model.Results{Poll: poll, Counts: make(map[string]int64, len(poll.Options)), Percentages: make(map[string]float64, len(poll.Options))}
	for _, option := range poll.Options {
		result.Counts[option.ID] = 0
	}
	return result
}

func calculatePercentages(result *model.Results, poll model.Poll) {
	for _, option := range poll.Options {
		if result.TotalVotes == 0 {
			result.Percentages[option.ID] = 0
		} else {
			result.Percentages[option.ID] = float64(result.Counts[option.ID]) * 100 / float64(result.TotalVotes)
		}
	}
	result.Poll.TotalVotes = result.TotalVotes
}

var ErrDuplicate = errors.New("vote already recorded")

func (s *Store) Vote(ctx context.Context, vote model.Vote) error {
	vote.CreatedAt = time.Now().UTC()
	vote.RedisSynced = false

	// Support existing callers that still provide only OptionID.
	if len(vote.OptionIDs) == 0 && vote.OptionID != "" {
		vote.OptionIDs = []string{vote.OptionID}
	}

	// Keep the legacy field populated for single-choice votes.
	if len(vote.OptionIDs) == 1 {
		vote.OptionID = vote.OptionIDs[0]
	} else {
		vote.OptionID = ""
	}

	_, err := s.Votes.InsertOne(ctx, vote)
	if mongo.IsDuplicateKeyError(err) {
		return ErrDuplicate
	}
	return err
}


func (s *Store) PendingVotes(ctx context.Context, limit int64) ([]model.Vote, error) {
	cursor, err := s.Votes.Find(ctx, bson.M{"redisSynced": bson.M{"$ne": true}}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}}).SetLimit(limit))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	votes := make([]model.Vote, 0)
	for cursor.Next(ctx) {
		var vote model.Vote
		if err = cursor.Decode(&vote); err != nil {
			return nil, err
		}
		votes = append(votes, vote)
	}
	return votes, cursor.Err()
}

func (s *Store) MarkVoteSynced(ctx context.Context, voteID string) error {
	_, err := s.Votes.UpdateOne(ctx, bson.M{"_id": voteID}, bson.M{"$set": bson.M{"redisSynced": true}})
	return err
}

// DeletePoll removes a poll and its associated votes from MongoDB.

 // DeletePoll removes a poll and its associated votes from MongoDB.

func (s *Store) DeletePoll(
    ctx context.Context,
    pollID string,
    ownerID string,
) error {
    filter := bson.M{
        "_id":     pollID,
        "ownerId": ownerID,
    }

    // Verify the poll exists and belongs to this owner.
    var poll model.Poll
    if err := s.Polls.FindOne(ctx, filter).Decode(&poll); err != nil {
        return err
    }

    // Delete associated votes using the same string poll ID.
    _, err := s.Votes.DeleteMany(ctx, bson.M{
        "pollId": pollID,
    })
    if err != nil {
        return err
    }

    // Delete the poll only if it still belongs to this owner.
    result, err := s.Polls.DeleteOne(ctx, filter)
    if err != nil {
        return err
    }

    if result.DeletedCount == 0 {
        return mongo.ErrNoDocuments
    }

    return nil
}
