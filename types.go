package main

import (
	"hypermedia/archiver"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

type app struct {
	mongo_client *mongo.Client
}

type Contact struct {
	ID     int               `json:"id"`
	First  string            `json:"first"`
	Last   string            `json:"last"`
	Email  string            `json:"email"`
	Phone  string            `json:"phone"`
	Errors map[string]string `json:"errors"`
}

type PageData struct {
	Contacts []Contact
	Query    string
	Page     int
	Archiver archiver.Archiver
}

// DB objects
type Contact_db struct {
	Username string `bson:"username"` // The username of the user who owns this contact
	ID       int    `bson:"id"`
	First    string `bson:"first"`
	Last     string `bson:"last"`
	Email    string `bson:"email"`
	Phone    string `bson:"phone"`
}

type User struct {
	Username string `bson:"username"`
	Password string `bson:"password"`
}

// Auth obejcts
type Session struct {
	Session_token string    `bson:"session_token"`
	Username      string    `bson:"username"`
	Expiry        time.Time `bson:"expiry"`
}

func (s Session) is_expired() bool {
	return s.Expiry.Before(time.Now())
}
