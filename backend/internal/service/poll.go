package service

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"livepoll/backend/internal/auth"
	"livepoll/backend/internal/live"
	"livepoll/backend/internal/model"
	"livepoll/backend/internal/store"
	"livepoll/backend/internal/validation"
)

var (
	ErrPollNotFound  = errors.New("poll not found")
	ErrPollID        = errors.New("invalid poll ID")
	ErrPollClosed    = errors.New("poll is closed")
	ErrInvalidChoice = errors.New("invalid option")
	ErrAlreadyVoted  = errors.New("you have already voted in this poll")
	ErrNotOwner      = errors.New("poll not found")
)

type PollService struct {
	Store *store.Store
	Live  *live.Live
}

type VoteOutcome struct {
	VoteID          string
	Results         model.Results
	PendingRealtime bool
}

type PollSettings struct {
	ClosesAt      *time.Time
	Anonymous     bool
	AllowMultiple bool
	MaxSelections int
}

func NewPoll(s *store.Store, l *live.Live) *PollService {
	return &PollService{Store: s, Live: l}
}

func (s *PollService) Create(
	ctx context.Context,
	ownerID, question string,
	options []string,
	settings ...PollSettings,
) (model.Poll, error) {
	question, options, err := validation.Poll(question, options)
	if err != nil {
		return model.Poll{}, err
	}

	cfg := PollSettings{MaxSelections: 1}
	if len(settings) > 0 {
		cfg = settings[0]
	}

	if cfg.ClosesAt != nil && !cfg.ClosesAt.After(time.Now()) {
		return model.Poll{}, errors.New("closing time must be in the future")
	}
	if cfg.AllowMultiple {
		if cfg.MaxSelections < 2 || cfg.MaxSelections > len(options) {
			return model.Poll{}, errors.New("maximum selections must be between 2 and the number of options")
		}
	} else {
		cfg.MaxSelections = 1
	}

	pollID, err := auth.Token()
	if err != nil {
		return model.Poll{}, err
	}

	poll := model.Poll{
		ID:            pollID[:24],
		OwnerID:       ownerID,
		Question:      question,
		Status:        "open",
		CreatedAt:     time.Now().UTC(),
		ClosesAt:      cfg.ClosesAt,
		Anonymous:     cfg.Anonymous,
		AllowMultiple: cfg.AllowMultiple,
		MaxSelections: cfg.MaxSelections,
	}

	for _, option := range options {
		optionID, tokenErr := auth.Token()
		if tokenErr != nil {
			return model.Poll{}, tokenErr
		}
		poll.Options = append(poll.Options, model.Option{
			ID:   optionID[:16],
			Text: option,
		})
	}

	if _, err = s.Store.Polls.InsertOne(ctx, poll); err != nil {
		return model.Poll{}, err
	}
	return poll, nil
}

func (s *PollService) List(ctx context.Context, ownerID string) ([]model.Poll, error) {
	cursor, err := s.Store.Polls.Find(
		ctx,
		bson.M{"ownerId": ownerID},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(100),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	polls := make([]model.Poll, 0)
	for cursor.Next(ctx) {
		var poll model.Poll
		if err = cursor.Decode(&poll); err != nil {
			return nil, err
		}
		poll.TotalVotes, err = s.Store.Count(ctx, poll.ID)
		if err != nil {
			return nil, err
		}
		polls = append(polls, poll)
	}
	return polls, cursor.Err()
}

func (s *PollService) Get(ctx context.Context, id string) (model.Poll, error) {
	if !validation.PollID(id) {
		return model.Poll{}, ErrPollID
	}

	poll, err := s.Store.GetPoll(ctx, id)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return model.Poll{}, ErrPollNotFound
	}
	if err != nil {
		return model.Poll{}, err
	}

	poll.TotalVotes, err = s.Store.Count(ctx, poll.ID)
	if err != nil {
		return model.Poll{}, err
	}
	return poll, nil
}

func (s *PollService) HasVoted(ctx context.Context, pollID, voterHash string) bool {
	if voterHash == "" {
		return false
	}
	count, _ := s.Store.Votes.CountDocuments(
		ctx,
		bson.M{"pollId": pollID, "voterId": voterHash},
	)
	return count > 0
}

func (s *PollService) Results(ctx context.Context, poll model.Poll) (model.Results, error) {
	durable, err := s.Store.Result(ctx, poll)
	if err != nil {
		return durable, err
	}

	cached, cacheErr := s.Live.Snapshot(ctx, poll.ID)
	if cacheErr != nil || !sameCounts(durable, cached) {
		rebuilt, votes, snapshotErr := s.Store.RebuildSnapshot(ctx, poll)
		if snapshotErr == nil {
			durable = rebuilt
			_ = s.Live.Seed(ctx, durable, votes)
			cached, cacheErr = s.Live.Snapshot(ctx, poll.ID)
		}
	}

	if cacheErr == nil && sameCounts(durable, cached) {
		durable.TotalVotes = cached.Total
		durable.Counts = cached.Counts
		for _, option := range poll.Options {
			if cached.Total == 0 {
				durable.Percentages[option.ID] = 0
			} else {
				durable.Percentages[option.ID] =
					float64(cached.Counts[option.ID]) * 100 / float64(cached.Total)
			}
		}
	}

	durable.Poll.TotalVotes = durable.TotalVotes
	return durable, nil
}

func sameCounts(durable model.Results, cached live.CounterSnapshot) bool {
	if durable.TotalVotes != cached.Total ||
		len(durable.Counts) != len(cached.Counts) {
		return false
	}
	for id, count := range durable.Counts {
		if cached.Counts[id] != count {
			return false
		}
	}
	return true
}

// Vote retains compatibility with existing single-choice callers.
func (s *PollService) Vote(
	ctx context.Context,
	pollID, optionID, voterHash string,
) (VoteOutcome, error) {
	return s.VoteOptions(ctx, pollID, []string{optionID}, voterHash)
}

func (s *PollService) VoteOptions(
	ctx context.Context,
	pollID string,
	optionIDs []string,
	voterHash string,
) (VoteOutcome, error) {
	poll, err := s.Get(ctx, pollID)
	if err != nil {
		return VoteOutcome{}, err
	}

	if poll.Status != "open" ||
		(poll.ClosesAt != nil && !time.Now().Before(*poll.ClosesAt)) {
		return VoteOutcome{}, ErrPollClosed
	}

	if voterHash == "" {
		return VoteOutcome{}, errors.New("voter identity is required")
	}

	if len(optionIDs) == 0 {
		return VoteOutcome{}, ErrInvalidChoice
	}

	if !poll.AllowMultiple && len(optionIDs) != 1 {
		return VoteOutcome{}, ErrInvalidChoice
	}

	maxSelections := poll.MaxSelections
	if maxSelections <= 0 {
		maxSelections = 1
	}
	if len(optionIDs) > maxSelections {
		return VoteOutcome{}, ErrInvalidChoice
	}

	validOptions := make(map[string]bool, len(poll.Options))
	for _, option := range poll.Options {
		validOptions[option.ID] = true
	}

	seen := make(map[string]bool, len(optionIDs))
	for _, optionID := range optionIDs {
		if !validOptions[optionID] || seen[optionID] {
			return VoteOutcome{}, ErrInvalidChoice
		}
		seen[optionID] = true
	}

	voteID, err := auth.Token()
	if err != nil {
		return VoteOutcome{}, err
	}

	vote := model.Vote{
		ID:        voteID,
		PollID:    poll.ID,
		VoterID:   voterHash,
		OptionIDs: optionIDs,
	}
	if len(optionIDs) == 1 {
		vote.OptionID = optionIDs[0]
	}

	if err = s.Store.Vote(ctx, vote); err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			return VoteOutcome{}, ErrAlreadyVoted
		}
		return VoteOutcome{}, err
	}

	pending := s.processVote(ctx, vote, poll)
	results, err := s.Results(ctx, poll)
	if err != nil {
		return VoteOutcome{VoteID: voteID, PendingRealtime: true}, err
	}

	return VoteOutcome{
		VoteID:          voteID,
		Results:         results,
		PendingRealtime: pending,
	}, nil
}

func (s *PollService) SetStatus(
	ctx context.Context,
	pollID, ownerID, status string,
) (model.Poll, error) {
	if !validation.PollID(pollID) {
		return model.Poll{}, ErrPollID
	}
	if status != "open" && status != "closed" {
		return model.Poll{}, errors.New("invalid poll status")
	}

	update, err := s.Store.Polls.UpdateOne(
		ctx,
		bson.M{"_id": pollID, "ownerId": ownerID},
		bson.M{"$set": bson.M{"status": status}},
	)
	if err != nil {
		return model.Poll{}, err
	}
	if update.MatchedCount == 0 {
		return model.Poll{}, ErrNotOwner
	}

	poll, err := s.Store.GetPoll(ctx, pollID)
	if err != nil {
		return model.Poll{}, err
	}
	poll.TotalVotes, err = s.Store.Count(ctx, poll.ID)
	if err != nil {
		return model.Poll{}, err
	}

	_ = s.Live.Publish(ctx, poll.ID, map[string]any{
		"type":   "status",
		"status": status,
		"poll":   poll,
	})
	return poll, nil
}




func (s *PollService) processVote(
	ctx context.Context,
	vote model.Vote,
	poll model.Poll,
) bool {
	optionIDs := vote.OptionIDs
	if len(optionIDs) == 0 && vote.OptionID != "" {
		optionIDs = []string{vote.OptionID}
	}
	if len(optionIDs) == 0 {
		return true
	}

	// Process the complete ballot atomically in Redis.
	if err := s.Live.VoteOptions(ctx, poll.ID, vote.ID, optionIDs); err != nil {
		return true
	}

	result, err := s.Results(ctx, poll)
	if err != nil {
		return true
	}

	event := map[string]any{
		"type":        "vote",
		"voteId":      vote.ID,
		"pollId":      poll.ID,
		"totalVotes":  result.TotalVotes,
		"counts":      result.Counts,
		"percentages": result.Percentages,
	}

	if err := s.Live.Publish(ctx, poll.ID, event); err != nil {
		return true
	}
	if err := s.Store.MarkVoteSynced(ctx, vote.ID); err != nil {
		return true
	}

	return false
}

func (s *PollService) ReconcilePending(ctx context.Context) {
	votes, err := s.Store.PendingVotes(ctx, 100)
	if err != nil {
		return
	}

	for _, vote := range votes {
		poll, err := s.Store.GetPoll(ctx, vote.PollID)
		if err != nil {
			continue
		}
		s.processVote(ctx, vote, poll)
	}
}