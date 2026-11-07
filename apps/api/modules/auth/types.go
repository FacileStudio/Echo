package auth

// UserResponse is the public shape of an account.
type UserResponse struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Picture   string `json:"picture"`
	Locale    string `json:"locale"`
	IsAdmin   bool   `json:"is_admin"`
	CreatedAt string `json:"created_at"`
}

// MeResponse is returned by /auth/me.
type MeResponse struct {
	User UserResponse `json:"user"`
}
