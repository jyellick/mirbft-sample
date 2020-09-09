module github.com/guoger/mir-sample

go 1.13

replace github.com/IBM/mirbft => github.com/jyellick/mirbft v0.0.0-20200909045218-ddbbff6cde3f

require (
	github.com/IBM/mirbft v0.0.0-20200820193629-05a8c61dd0f9
	github.com/golang/protobuf v1.4.1
	github.com/perlin-network/noise v1.1.3
	github.com/pkg/errors v0.8.1
	github.com/stretchr/testify v1.6.1
	go.uber.org/zap v1.14.1
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/yaml.v2 v2.2.2
)
