module github.com/jyellick/mirbft-sample

go 1.14

// replace github.com/IBM/mirbft => github.com/jyellick/mirbft v0.0.0-20201008041652-edabc2501273

require (
	github.com/IBM/mirbft v0.0.0-20210128153015-7e18f493689f
	github.com/golang/protobuf v1.4.1
	github.com/ibm/mirbft v0.0.0-20190415184034-d1829758e6fc // indirect
	github.com/perlin-network/noise v1.1.3
	github.com/pkg/errors v0.8.1
	github.com/stretchr/testify v1.6.1
	go.uber.org/zap v1.14.1
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/yaml.v2 v2.2.2
)
