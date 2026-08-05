package observability

// Version is the build version stamped at compile time (ldflags -X). It is
// attached to every structured log line; the default is "dev".
var Version = "dev"
