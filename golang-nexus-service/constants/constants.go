package constants

// Error codes for Nexus API
const (
	// OData parsing errors (50000-50099)
	ErrorCodeODataParsingError     = 50019 // Failed to parse OData query parameters
	ErrorCodeODataPropertyNotFound = 50020 // Unknown property in OData query
	ErrorCodeODataInvalidSyntax    = 50021 // Invalid OData query syntax
	ErrorCodeODataEvaluationError  = 50022 // Failed to evaluate OData query

	// Internal errors (50100-50199)
	ErrorCodeInternalError = 50100 // Internal server error
)

// Error namespace and prefix for AppMessage
const (
	EnglishLocale    = "en-US"
	NexusNamespace   = "nexus"
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
