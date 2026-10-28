package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"livepoll/backend/internal/auth"
	"livepoll/backend/internal/model"
	"livepoll/backend/internal/store"
	"livepoll/backend/internal/validation"
)

var (
	ErrDuplicateEmail = errors.New("email already registered")
	ErrUnauthorized   = errors.New("authentication required")
)

type AuthService struct{ Store *store.Store }

type SessionTokens struct{ Session, CSRF string }

func NewAuth(s *store.Store) *AuthService { return &AuthService{Store: s} }

func (s *AuthService) Signup(ctx context.Context, name, email, password string) (model.User, error) {
	name, email, err := validation.Account(name, email, password)
	if err != nil {
		return model.User{}, err
	}
	passwordHash, err := auth.Hash(password)
	if err != nil {
		return model.User{}, err
	}
	userID, err := auth.Token()
	if err != nil {
		return model.User{}, err
	}
	user := model.User{ID: userID, Name: name, Email: email, PasswordHash: passwordHash, CreatedAt: time.Now().UTC()}
	if _, err = s.Store.Users.InsertOne(ctx, user); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return model.User{}, ErrDuplicateEmail
		}
		return model.User{}, err
	}
	return user, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (model.User, error) {
	email, err := validation.Credentials(email, password)
	if err != nil {
		return model.User{}, err
	}
	var user model.User
	if err = s.Store.Users.FindOne(ctx, bson.M{"email": email}).Decode(&user); err != nil || !auth.Check(user.PasswordHash, password) {
		return model.User{}, validation.ErrInvalidCredentials
	}
	return user, nil
}

func (s *AuthService) CreateSession(ctx context.Context, user model.User) (SessionTokens, error) {
	sessionID, err := auth.Token()
	if err != nil {
		return SessionTokens{}, err
	}
	csrfID, err := auth.Token()
	if err != nil {
		return SessionTokens{}, err
	}
	session := model.Session{ID: sessionID, UserID: user.ID, ExpiresAt: time.Now().UTC().Add(7 * 24 * time.Hour)}
	if _, err = s.Store.Sessions.InsertOne(ctx, session); err != nil {
		return SessionTokens{}, err
	}
	return SessionTokens{Session: sessionID, CSRF: csrfID}, nil
}

func (s *AuthService) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	_, err := s.Store.Sessions.DeleteOne(ctx, bson.M{"_id": token})
	return err
}

func (s *AuthService) Current(ctx context.Context, token string) (model.User, error) {
	if strings.TrimSpace(token) == "" {
		return model.User{}, ErrUnauthorized
	}
	user, err := auth.UserForSession(ctx, s.Store, token)
	if err != nil {
		return model.User{}, ErrUnauthorized
	}
	return user, nil
}
