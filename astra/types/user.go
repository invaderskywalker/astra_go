// astra/types/create_user_request.go (new)
package types

type CreateUserRequest struct {
	Username string  `json:"username"`
	Email    string  `json:"email"`
	FullName *string `json:"full_name,omitempty"`
}
