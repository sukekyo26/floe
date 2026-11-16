package files_test

import "floe/internal/usererr"

// errLinkToNowhere is what OpenAppend of a link to nowhere returns: opened as the link itself, it is no regular file.
var errLinkToNowhere = usererr.ErrNotFile
