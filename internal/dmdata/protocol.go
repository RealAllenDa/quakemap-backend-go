package dmdata

import "time"

type TokenRequest struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	GrantType    string `json:"grant_type"`
	RefreshToken string `json:"refresh_token"`
}

type TokenResponse struct {
	AccessToken      string `json:"access_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int64  `json:"expires_in"`
	Scope            string `json:"scope"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type GenericResponse struct {
	ResponseID   string    `json:"responseId"`
	ResponseTime time.Time `json:"responseTime"`
	Status       string    `json:"status"`
}

type APIError struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

type SocketRequest struct {
	Classifications []string `json:"classifications"`
	Types           []string `json:"types"`
	AppName         string   `json:"appName"`
}

type WebSocketInfo struct {
	ID         int64    `json:"id"`
	URL        string   `json:"url"`
	Protocol   []string `json:"protocol"`
	Expiration int64    `json:"expiration"`
}

type SocketResponse struct {
	GenericResponse
	Ticket          string        `json:"ticket"`
	WebSocket       WebSocketInfo `json:"websocket"`
	Classifications []string      `json:"classifications"`
	Test            string        `json:"test"`
	Types           []string      `json:"types"`
	Formats         []string      `json:"formats"`
	Error           *APIError     `json:"error"`
}

type Envelope struct {
	Type string `json:"type"`
}

type SocketStart struct {
	Type            string    `json:"type"`
	SocketID        int64     `json:"socketId"`
	Classifications []string  `json:"classifications"`
	Types           []string  `json:"types"`
	Test            string    `json:"test"`
	Formats         []string  `json:"formats"`
	Time            time.Time `json:"time"`
}

type SocketPing struct {
	Type   string `json:"type"`
	PingID string `json:"pingId"`
}

type SocketPong struct {
	Type   string `json:"type"`
	PingID string `json:"pingId"`
}

type SocketError struct {
	Type  string `json:"type"`
	Error string `json:"error"`
	Code  int    `json:"code"`
	Close bool   `json:"close"`
}
