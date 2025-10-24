package auth

import (
	"log"

	"layeh.com/radius"
	"layeh.com/radius/rfc2865"
)

// Handler handles RADIUS authentication requests
type Handler struct {
	Secret []byte
}

// NewHandler creates a new authentication handler
func NewHandler(secret []byte) *Handler {
	return &Handler{
		Secret: secret,
	}
}

// Handle processes authentication requests
func (h *Handler) Handle(w radius.ResponseWriter, r *radius.Request) {
	username := rfc2865.UserName_GetString(r.Packet)
	password := rfc2865.UserPassword_GetString(r.Packet)

	log.Printf("[AUTH] Received Access-Request from %v for user: %s", r.RemoteAddr, username)

	var code radius.Code
	if username == "tim" && password == "12345" {
		code = radius.CodeAccessAccept
		log.Printf("[AUTH] Access granted for user: %s", username)
	} else {
		code = radius.CodeAccessReject
		log.Printf("[AUTH] Access denied for user: %s", username)
	}

	w.Write(r.Response(code))
}
