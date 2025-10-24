package auth

import (
	"log"

	"layeh.com/radius"
	"layeh.com/radius/rfc2865"
)

// Handler handles RADIUS authentication requests
type Handler struct {
	Secret          []byte
	UserCredentials map[string]string
}

// NewHandler creates a new authentication handler
func NewHandler(secret []byte, userCredentials map[string]string) *Handler {
	return &Handler{
		Secret:          secret,
		UserCredentials: userCredentials,
	}
}

// Handle processes authentication requests
func (h *Handler) Handle(w radius.ResponseWriter, r *radius.Request) {
	username := rfc2865.UserName_GetString(r.Packet)
	password := rfc2865.UserPassword_GetString(r.Packet)

	log.Printf("[AUTH] Received Access-Request from %v for user: %s", r.RemoteAddr, username)

	var code radius.Code

	// Check if username exists and password matches
	if expectedPassword, exists := h.UserCredentials[username]; exists && expectedPassword == password {
		code = radius.CodeAccessAccept
		log.Printf("[AUTH] Access granted for user: %s", username)
	} else {
		code = radius.CodeAccessReject
		if !exists {
			log.Printf("[AUTH] Access denied for user: %s (user not found)", username)
		} else {
			log.Printf("[AUTH] Access denied for user: %s (invalid password)", username)
		}
	}

	w.Write(r.Response(code))
}
