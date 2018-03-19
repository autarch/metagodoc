package repository

type ActivityStatus int

const (
	Active          ActivityStatus = iota
	DeadEndFork                    // Forks with no commits
	QuickFork                      // Forks with less than 3 commits, all within a week from creation
	NoRecentCommits                // No commits for ExpiresAfter

	// No commits for ExpiresAfter and no imports.
	// This is a status derived from NoRecentCommits and the imports count information in the db.
	Inactive
)

var statusMap = map[ActivityStatus]string{
	Active:          "active",
	DeadEndFork:     "dead-end-fork",
	QuickFork:       "quick-fork",
	NoRecentCommits: "no-recent-commits",
	Inactive:        "inactive",
}

func (st ActivityStatus) String() string {
	if v, ok := statusMap[st]; ok {
		return v
	}
	return "invalid"
}
