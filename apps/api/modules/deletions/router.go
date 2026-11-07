package deletions

import (
	"net/http"

	"github.com/FacileStudio/porte/directory"
	"github.com/go-chi/chi/v5"
)

// RegisterRoutes mounts Registre's signed user.deleted push at
// directory.DeletionsPath. It sits outside the session and CSRF middleware:
// the handler verifies the notice's signature itself.
func RegisterRoutes(router chi.Router, deletions *Deletions) {
	router.Method(http.MethodPost, directory.DeletionsPath, deletions.Handler())
}
