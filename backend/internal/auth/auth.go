package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"go.mongodb.org/mongo-driver/bson"
	"golang.org/x/crypto/bcrypt"
	"livepoll/backend/internal/model"
	"livepoll/backend/internal/store"
	"time"
)

func Hash(p string) (string, error) {
	b, e := bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost)
	return string(b), e
}
func Check(hash, p string) bool { return bcrypt.CompareHashAndPassword([]byte(hash), []byte(p)) == nil }
func Token() (string, error) {
	b := make([]byte, 32)
	_, e := rand.Read(b)
	return hex.EncodeToString(b), e
}
func UserForSession(ctx context.Context, s *store.Store, token string) (model.User, error) {
	var sess model.Session
	var u model.User
	e := s.Sessions.FindOne(ctx, bson.M{"_id": token, "expiresAt": bson.M{"$gt": time.Now().UTC()}}).Decode(&sess)
	if e != nil {
		return u, e
	}
	e = s.Users.FindOne(ctx, bson.M{"_id": sess.UserID}).Decode(&u)
	return u, e
}
