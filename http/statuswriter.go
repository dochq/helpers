package http

import (
	basehttp "net/http"
)

// ----------------------------------------------------------------------------
// there's really no way of doing this without creating my own response writer
// and passing it down each time for each endpoint
//
// https://www.reddit.com/r/golang/comments/7p35s4/how_do_i_get_the_response_status_for_my_middleware/
// ----------------------------------------------------------------------------

type statusWriter struct {
	basehttp.ResponseWriter
	status int
	length int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	n, err := w.ResponseWriter.Write(b)
	w.length += n
	return n, err
}

func GetResponseWriter(w interface{}) basehttp.ResponseWriter {
	if out, ok := w.(*statusWriter); ok {
		return out.ResponseWriter
	}

	return nil
}
