package archiver

import "time"

type Status string

const (
	StatusWaiting  Status = "Waiting"
	StatusRunning  Status = "Running"
	StatusComplete Status = "Complete"
)

type Archiver struct {
	Status   Status
	Progress float64
	FilePath string
}

func (a *Archiver) Run() {
	if a.Status != StatusWaiting {
		return
	}
	a.Status = StatusRunning
	// Launch the archiving process asynchronously

	go a.doArchive()
}

func (a *Archiver) Reset() {
	a.Status = StatusWaiting
	a.Progress = 0
	a.FilePath = ""
}

func (a *Archiver) ArchiveFile() string {
	if a.Status != StatusComplete {
		return ""
	}
	return a.FilePath
}

// Is not diretly imported
func (a *Archiver) doArchive() {
	// Simulate archive progress
	for i := 0; i <= 100; i++ {
		time.Sleep(50 * time.Millisecond)
		a.Progress = float64(i) / 100
	}
	a.Status = StatusComplete
	a.FilePath = "/path/to/archive.zip"
}

var userArchivers = make(map[string]*Archiver)

func GetArchiverForUser(userID string) *Archiver {
	if a, exists := userArchivers[userID]; exists {
		return a
	}
	newArchiver := &Archiver{Status: StatusWaiting}
	userArchivers[userID] = newArchiver
	return newArchiver
}
