package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"dendro-chronology-workbench/internal/domain"
)

const maxBodyBytes = 2 << 20

type apiError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Field   string `json:"field,omitempty"`
	} `json:"error"`
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	return decodeJSONLimit(w, r, target, maxBodyBytes)
}

func decodeJSONLimit(w http.ResponseWriter, r *http.Request, target any, limit int64) error {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		if errors.Is(err, io.EOF) {
			return domain.NewError(domain.ErrValidation, "body", "请求体不能为空")
		}
		return domain.NewError(domain.ErrValidation, "body", "JSON 请求无效：%v", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return domain.NewError(domain.ErrValidation, "body", "请求体只能包含一个 JSON 对象")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeRaw(w http.ResponseWriter, status int, body json.RawMessage, replayed bool) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if replayed {
		w.Header().Set("Idempotent-Replayed", "true")
	}
	w.WriteHeader(status)
	_, _ = w.Write(body)
	_, _ = w.Write([]byte("\n"))
}

func writeError(w http.ResponseWriter, status int, code, message, field string) {
	var out apiError
	out.Error.Code = code
	out.Error.Message = message
	out.Error.Field = field
	writeJSON(w, status, out)
}

func handleError(w http.ResponseWriter, err error) {
	if de, ok := err.(*domain.DomainError); ok {
		status := http.StatusBadRequest
		switch de.Code {
		case domain.ErrNotFound:
			status = http.StatusNotFound
		case domain.ErrConflict:
			status = http.StatusConflict
		case domain.ErrForbidden:
			status = http.StatusForbidden
		case domain.ErrState:
			status = http.StatusUnprocessableEntity
		case domain.ErrIntegrity:
			status = http.StatusInternalServerError
		}
		writeError(w, status, string(de.Code), de.Message, de.Field)
		return
	}
	writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "服务内部错误", "")
}
