package icons

import "regexp"

// aws is the AWS Architecture Icons: 48px services and resources and 32px groups, and a variant for
// dark backgrounds of an icon without a light one. The package's URL holds its date and a hash and
// changes every quarter.
var aws = kit{
	name:  "aws",
	title: "AWS",
	page:  "https://aws.amazon.com/architecture/icons/",
	host:  "aws.amazon.com",
	size:  "14MB",
	terms: false,
	packages: []pack{{
		what:   "the Icon package zip",
		link:   regexp.MustCompile(`^https://[^"'\s<>]+/Icon-package_\d{8}[^"'\s<>/?#]*\.zip$`),
		iconOf: awsIconOf,
	}},
	version: regexp.MustCompile(`Icon-package_(\d{8})`),
	short: map[string][]string{
		"acm":   {"aws-certificate-manager"},
		"alb":   {"res/elastic-load-balancing-application-load-balancer"},
		"apigw": {"amazon-api-gateway"},
		"asg":   {"aws-auto-scaling", "amazon-ec2-auto-scaling"},
		"cdn":   {"amazon-cloudfront"},
		"clb":   {"res/elastic-load-balancing-classic-load-balancer"},
		"ddb":   {"amazon-dynamodb"},
		"dms":   {"aws-database-migration-service"},
		"ebs":   {"amazon-elastic-block-store"},
		"ecr":   {"amazon-elastic-container-registry"},
		"ecs":   {"amazon-elastic-container-service"},
		"eks":   {"amazon-elastic-kubernetes-service"},
		"elb":   {"elastic-load-balancing"},
		"iam":   {"aws-identity-and-access-management"},
		"igw":   {"res/amazon-vpc-internet-gateway"},
		"k8s":   {"amazon-elastic-kubernetes-service"},
		"kms":   {"aws-key-management-service"},
		"msk":   {"amazon-managed-streaming-for-apache-kafka"},
		"nat":   {"res/amazon-vpc-nat-gateway"},
		"nlb":   {"res/elastic-load-balancing-network-load-balancer"},
		"r53":   {"amazon-route-53"},
		"ram":   {"aws-resource-access-manager"},
		"s3":    {"amazon-simple-storage-service"},
		"ses":   {"amazon-simple-email-service"},
		"sfn":   {"aws-step-functions"},
		"sns":   {"amazon-simple-notification-service"},
		"sqs":   {"amazon-simple-queue-service"},
		"ssm":   {"aws-systems-manager"},
		"sso":   {"aws-iam-identity-center"},
		"vpc":   {"amazon-virtual-private-cloud"},
	},
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
		return named(packed{version: m[1], name: "", category: m[2], dark: m[4] != ""}, "", m[3])
	}
	if m := awsGroupRe.FindStringSubmatch(path); m != nil {
		return named(packed{version: m[1], name: "", category: "Group", dark: m[3] != ""}, "group/", m[2])
	}
	if m := awsResRe.FindStringSubmatch(path); m != nil {
		return named(packed{version: m[1], name: "", category: m[2], dark: m[4] == "Dark"}, "res/", m[3])
	}
	return packed{version: "", name: "", category: "", dark: false}, false
}
