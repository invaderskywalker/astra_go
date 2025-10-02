// astra/types/user.go
package types

type CreateUserRequest struct {
	Username string  `json:"username"`
	Email    string  `json:"email"`
	FullName *string `json:"full_name,omitempty"`
}
