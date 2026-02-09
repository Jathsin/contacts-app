package archiver

import (
	"encoding/json"
	"fmt"
	"time"
)

type Archiver struct {
	Start time.Time
	Limit time.Duration
	State string
}

var default_archiver = new_archiver(5 * time.Second)

// state waiting, running, complete
func new_archiver(limit time.Duration) *Archiver {
	return &Archiver{
		Start: time.Time{},
		Limit: limit,
		State: "waiting",
	}
}

func Get() *Archiver {
	return default_archiver
}

func (a *Archiver) Status() string {
	if a.State == "running" {
		if a.Progress() >= 1.0 {
			a.State = "complete"
		}

	}
	return a.State
}

func (a *Archiver) Progress() float64 {
	if a.State != "running" {
		return 0.0
	}
	now := time.Now()
	elapsed := now.Sub(a.Start)

	return float64(elapsed) / float64(a.Limit)
}

func (a *Archiver) Run() {
	if a.State != "waiting" {
		fmt.Println("archiver: cannot run, not in waiting state")
		return
	}

	a.State = "running"
	a.Start = time.Now()
}

func (a *Archiver) Reset() {
	a.State = "waiting"
	a.Start = time.Time{}
}

func (a *Archiver) Archive_file(data interface{}) interface{} {
	data, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Println("archiver: failed to marshal contact list", "error", err)
		return nil
	}
	return data
}
