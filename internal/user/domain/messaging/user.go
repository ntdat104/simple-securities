package messaging

import "fmt"

type UserRegistered struct {
	UserID    uint64
	UserUUID  string
	Email     string
	Timestamp int64
	RequestID string
}

func (e UserRegistered) EventName() string { return "user.registered" }
func (e UserRegistered) EventKey() string  { return fmt.Sprintf("%d", e.UserID) }

type UserLoggedIn struct {
	UserID    uint64
	UserUUID  string
	Email     string
	Timestamp int64
	RequestID string
}

func (e UserLoggedIn) EventName() string { return "user.logged_in" }
func (e UserLoggedIn) EventKey() string  { return fmt.Sprintf("%d", e.UserID) }

type UserProfileViewed struct {
	UserID    uint64
	UserUUID  string
	Email     string
	Timestamp int64
	RequestID string
}

func (e UserProfileViewed) EventName() string { return "user.profile_viewed" }
func (e UserProfileViewed) EventKey() string  { return fmt.Sprintf("%d", e.UserID) }
