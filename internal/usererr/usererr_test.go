package usererr_test

import (
	"testing"

	"github.com/sukekyo26/floe/internal/usererr"
)

func TestNew(t *testing.T) {
	t.Parallel()

	if e := usererr.New("msg", "理由"); e.Error() != "msg" || e.Ja != "理由" || e.Conflict {
		t.Errorf("New = %+v", e)
	}
	if e := usererr.New("msg", ""); e.Ja != "msg" {
		t.Errorf("New with no ja = %+v, want Ja to fall back to msg", e)
	}
}
