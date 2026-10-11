package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const maxJSONBodyBytes = 1 << 20

// decodeJSONRequest проверяет структуру JSON и ограничивает размер тела запроса.
func decodeJSONRequest(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		writeJSONDecodeError(w, err)
		return false
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
		} else {
			writeJSONDecodeError(w, err)
		}
		return false
	}

	return true
}

func writeJSONDecodeError(w http.ResponseWriter, err error) {
	var sizeErr *http.MaxBytesError
	if errors.As(err, &sizeErr) {
		writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
		return
	}

	writeError(w, http.StatusBadRequest, "invalid JSON body")
}
