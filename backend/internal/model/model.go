
package model

import "time"

type User struct {
	ID           string    `bson:"_id" json:"id"`
	Name         string    `bson:"name" json:"name"`
	Email        string    `bson:"email" json:"email"`
	PasswordHash string    `bson:"passwordHash" json:"-"`
	CreatedAt    time.Time `bson:"createdAt" json:"createdAt"`
}

type Option struct {
	ID   string `bson:"id" json:"id"`
	Text string `bson:"text" json:"text"`
}

type Poll struct {
	ID             string    `bson:"_id" json:"id"`
	OwnerID        string    `bson:"ownerId" json:"ownerId,omitempty"`
	Question       string    `bson:"question" json:"question"`
	Options        []Option  `bson:"options" json:"options"`
	Status         string    `bson:"status" json:"status"`
	CreatedAt      time.Time `bson:"createdAt" json:"createdAt"`
	ClosesAt       *time.Time `bson:"closesAt,omitempty" json:"closesAt,omitempty"`
	Anonymous      bool      `bson:"anonymous" json:"anonymous"`
	AllowMultiple  bool      `bson:"allowMultiple" json:"allowMultiple"`
	MaxSelections  int       `bson:"maxSelections" json:"maxSelections"`
	TotalVotes     int64     `bson:"-" json:"totalVotes"`
	HasVoted       bool      `bson:"-" json:"hasVoted,omitempty"`
}

type Vote struct {
	ID          string    `bson:"_id" json:"-"`
	PollID      string    `bson:"pollId" json:"-"`
	VoterID     string    `bson:"voterId" json:"-"`
	OptionID    string    `bson:"optionId,omitempty" json:"-"`
	OptionIDs   []string  `bson:"optionIds,omitempty" json:"-"`
	RedisSynced bool      `bson:"redisSynced" json:"-"`
	CreatedAt   time.Time `bson:"createdAt" json:"-"`
}

type Session struct {
	ID        string    `bson:"_id"`
	UserID    string    `bson:"userId"`
	ExpiresAt time.Time `bson:"expiresAt"`
}

type Results struct {
	Poll        Poll               `json:"poll"`
	TotalVotes  int64              `json:"totalVotes"`
	Counts      map[string]int64   `json:"counts"`
	Percentages map[string]float64 `json:"percentages"`
}
