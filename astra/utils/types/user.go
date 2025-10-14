// astra/types/user.go
package types

type UpdateUserRequest struct {
	Username *string `json:"username,omitempty"`
	Email    *string `json:"email,omitempty"`
	FullName *string `json:"full_name,omitempty"`
	ImageURL *string `json:"image_url,omitempty"`
}

type CreateUserRequest struct {
	Username string  `json:"username"`
	Email    string  `json:"email"`
	FullName *string `json:"full_name,omitempty"`
	ImageURL *string `json:"image_url,omitempty"`
}
