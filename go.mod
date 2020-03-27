module github.com/guoger/mir-sample

go 1.13

replace github.com/IBM/mirbft => github.com/jyellick/mirbft v0.0.0-20200326130436-cba4fae9f04c

require (
	github.com/IBM/mirbft v0.0.0-00010101000000-000000000000
	github.com/golang/protobuf v1.3.5
	github.com/onsi/ginkgo v1.8.0
	github.com/onsi/gomega v1.5.0
	github.com/perlin-network/noise v1.1.2
	github.com/pkg/errors v0.8.1
	go.uber.org/zap v1.14.1
	gopkg.in/yaml.v2 v2.2.2
)
