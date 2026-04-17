package constants

// Error codes for Nexus API
const (
	ErrorCodeODataParsingError = 50019
	ErrorCodeInternalError     = 50100
)

// AppMessage constants used when building gRPC error responses.
const (
	EnglishLocale    = "en_US"
	NexusErrorPrefix = "NEXUS"
)

// IDF configuration
const (
	IdfHost = "127.0.0.1"
	IdfPort = 2027
)

// StatsGW configuration
const (
	StatsGWHost = "127.0.0.1"
	StatsGWPort = 8084
)
