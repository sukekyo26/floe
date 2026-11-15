package kit

import "errors"

// Read and Check are read and check, for the tests of this package to read and check apart; FloeFailure is
// floeFailure, which no own thing reaches; PageMarkdown is the Markdown of floe's pages.
var (
	Read         = read
	Check        = check
	FloeFailure  = floeFailure
	PageMarkdown = markdown
)

// Compiled returns how many of k's templates were compiled again.
func Compiled(k *Kit) int { return int(k.compiled.Load()) }

// FailRebuilt makes k fail to compile its template name again, as floe would with a mistake of its own: before k is
// taken from, as read writes no map after it returns.
func FailRebuilt(k *Kit, name string) {
	k.rebuilt[name] = func() (*Template, error) { return nil, floeFailure(name, errors.New("broken")) } //nolint:err113 // a test's
}
