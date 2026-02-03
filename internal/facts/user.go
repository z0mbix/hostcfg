package facts

import (
	"os"
	"os/user"
	"strconv"
)

// UserFacts contains current user information
type UserFacts struct {
	Name string // Current username
	Home string // Home directory
	UID  string // User ID
	GID  string // Group ID
}

// gatherUserFacts collects user-related facts
func gatherUserFacts() (UserFacts, error) {
	facts := UserFacts{}

	// Get current user info
	currentUser, err := user.Current()
	if err != nil {
		// Fall back to environment variables and syscalls
		facts.Name = os.Getenv("USER")
		if facts.Name == "" {
			facts.Name = "unknown"
		}

		facts.Home, _ = os.UserHomeDir()
		if facts.Home == "" {
			facts.Home = os.Getenv("HOME")
		}

		facts.UID = strconv.Itoa(os.Getuid())
		facts.GID = strconv.Itoa(os.Getgid())

		return facts, nil
	}

	facts.Name = currentUser.Username
	facts.Home = currentUser.HomeDir
	facts.UID = currentUser.Uid
	facts.GID = currentUser.Gid

	return facts, nil
}
