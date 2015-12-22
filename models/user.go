package models

type UserList struct {
	Users []User `json:"users"`
}

type User struct {
	Name      string `json:"name"`
	AvatarUrl string `json:"photo_url"`
}
