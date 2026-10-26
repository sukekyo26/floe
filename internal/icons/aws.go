package icons

import "regexp"

// aws is the AWS Architecture Icons: 48px services and resources and 32px groups, and a variant for
// dark backgrounds of an icon without a light one. The package's URL holds its date and a hash and
// changes every quarter.
var aws = kit{
	name:    "aws",
	title:   "AWS",
	page:    "https://aws.amazon.com/architecture/icons/",
	host:    "aws.amazon.com",
	size:    "14MB",
	link:    regexp.MustCompile(`^https://[^"'\s<>]+/Icon-package_\d{8}[^"'\s<>/?#]*\.zip$`),
	version: regexp.MustCompile(`Icon-package_(\d{8})`),
	iconOf:  awsIconOf,
}

// A name part has no slash, so no entry of the zip can write outside the set's directory.
var (
	awsServiceRe = regexp.MustCompile(`^Architecture-Service-Icons_(\d{8})/Arch_([^/]+)/48/Arch_([^/]+?)(_Dark)?_48\.svg$`)
	awsGroupRe   = regexp.MustCompile(`^Architecture-Group-Icons_(\d{8})/([^/]+?)_32(_Dark)?\.svg$`)
	awsResRe     = regexp.MustCompile(`^Resource-Icons_(\d{8})/Res_([^/]+)/(?:Res_48_(?:Light|Dark)/)?` +
		`Res_([^/]+?)_48(?:_(Light|Dark))?\.svg$`)
)

func awsIconOf(path string) (packed, bool) {
	if m := awsServiceRe.FindStringSubmatch(path); m != nil {
		return packed{version: m[1], name: slug(m[3]), category: m[2], dark: m[4] != ""}, true
	}
	if m := awsGroupRe.FindStringSubmatch(path); m != nil {
		return packed{version: m[1], name: "group/" + slug(m[2]), category: "Group", dark: m[3] != ""}, true
	}
	if m := awsResRe.FindStringSubmatch(path); m != nil {
		return packed{version: m[1], name: "res/" + slug(m[3]), category: m[2], dark: m[4] == "Dark"}, true
	}
	return packed{version: "", name: "", category: "", dark: false}, false
}
