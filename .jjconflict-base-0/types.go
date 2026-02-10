package main

import (
	"hypermedia/archiver"
	"time"
)

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

//Auth obejcts

var sessions = map[string]session{}

type session struct {
	username string
	expiry   time.Time
}

func (s session) is_expired() bool {
	return s.expiry.Before(time.Now())
}
