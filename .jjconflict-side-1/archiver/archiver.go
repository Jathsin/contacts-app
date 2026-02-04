package archiver

import (
	"encoding/json"
	"fmt"
	"time"
)

type Archiver struct {
	start time.Time
	limit time.Duration
	state string
}

var default_archiver = new_archiver(5 * time.Second)

// state waiting, running, complete
func new_archiver(limit time.Duration) *Archiver {
	return &Archiver{
		start: time.Time{},
		limit: limit,
		state: "waiting",
	}
}

func Get() *Archiver {
	return default_archiver
}

func (a *Archiver) Status() string {
	if a.state == "running" {
		if a.Progress() >= 1.0 {
			a.state = "complete"
		}
	}
	return a.state
}

func (a *Archiver) Progress() float64 {
	if a.state != "running" {
		return 0.0
	}
	now := time.Now()
	elapsed := now.Sub(a.start)

	return float64(elapsed) / float64(a.limit)
}

func (a *Archiver) Run() {
	if a.state != "waiting" {
		fmt.Println("archiver: cannot run, not in waiting state")
		return
	}

	a.state = "running"
	a.start = time.Now()
}

func (a *Archiver) Reset() {
	a.state = "waiting"
	a.start = time.Time{}
}

func (a *Archiver) Archive_file(data interface{}) interface{} {
	data, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Println("archiver: failed to marshal contact list", "error", err)
		return nil
	}
	return data
}
