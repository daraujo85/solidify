// Users API — backend.
package api

import (
	"encoding/json"
	"net/http"
)

type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func HandleUsers(w http.ResponseWriter, r *http.Request) {
	u := User{ID: "1", Name: "fixture-user"}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(u)
}
