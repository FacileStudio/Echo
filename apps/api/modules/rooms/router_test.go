package rooms

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FacileStudio/Echo/apps/api/internal/middleware"
	"github.com/go-chi/chi/v5"
)

type fakeLimits struct {
	spent  []string
	refuse map[string]bool
}

func (f *fakeLimits) Allow(_ *http.Request, budget middleware.Budget) bool {
	f.spent = append(f.spent, budget.Scope)
	return !f.refuse[budget.Scope]
}

func post(router http.Handler, path string) int {
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"display_name":"Guest"}`)))
	return recorder.Code
}

func TestCreatingARoomSpendsTheStrictBudgetAndJoiningTheRoomsOwn(t *testing.T) {
	s := newTestService(t)
	if _, err := s.Create(context.Background(), "office", "office", nil); err != nil {
		t.Fatalf("create: %v", err)
	}
	limits := &fakeLimits{refuse: map[string]bool{"room-create": true}}
	router := chi.NewRouter()
	RegisterRoutes(router, s, &stubResolver{}, func(next http.Handler) http.Handler { return next }, limits)

	if code := post(router, "/rooms"); code != http.StatusTooManyRequests {
		t.Errorf("POST /rooms with the creation budget spent answered %d, want 429", code)
	}
	if code := post(router, "/rooms/junk-slug/token"); code != http.StatusTooManyRequests {
		t.Errorf("a token for an unknown slug with the creation budget spent answered %d, want 429", code)
	}
	if code := post(router, "/rooms/office/token"); code != http.StatusOK {
		t.Errorf("joining an existing room was held to the creation budget: %d", code)
	}
	if got, ok := s.BySlug(context.Background(), "junk-slug"); ok != nil || got != nil {
		t.Fatal("a refused token still created the room")
	}
	want := []string{"room-create", "room-create", "join:office"}
	if strings.Join(limits.spent, ",") != strings.Join(want, ",") {
		t.Fatalf("budgets spent %v, want %v", limits.spent, want)
	}
}
