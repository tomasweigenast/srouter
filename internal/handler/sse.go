package handler

import (
	"net/http"
	"time"
)

// zeroTime is used to clear a response write deadline, keeping SSE connections
// alive indefinitely (the server's WriteTimeout would otherwise close them).
var zeroTime time.Time

// sseHeaders sets the required headers for a Server-Sent Events response and
// disables the write deadline so the connection stays open past WriteTimeout.
// Returns false if the response writer does not support flushing.
func sseHeaders(w http.ResponseWriter) (http.Flusher, bool) {
	_ = http.NewResponseController(w).SetWriteDeadline(zeroTime)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	return flusher, ok
}
