package repository

type ActivityStatus string

const (
	Active          ActivityStatus = "active"
	DeadEndFork                    = "dead-end-fork"     // Forks with no commits
	QuickFork                      = "quick-fork"        // Forks with less than 3 commits, all within a week from creation
	NoRecentCommits                = "no-recent-commits" // No commits for ExpiresAfter

	// No commits for ExpiresAfter and no imports.
	// This is a status derived from NoRecentCommits and the imports count information in the db.
	Inactive = "inactive"
)
