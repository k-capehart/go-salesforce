module github.com/k-capehart/go-salesforce/v2

go 1.23.0

toolchain go1.23.5

require github.com/forcedotcom/go-soql v0.0.0-20220705175410-00f698360bee

require (
	github.com/go-viper/mapstructure/v2 v2.2.1
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/jszwec/csvutil v1.10.0
	github.com/spf13/afero v1.12.0
	k8s.io/apimachinery v0.32.1
)

require (
	github.com/go-logr/logr v1.4.2 // indirect
	golang.org/x/text v0.21.0 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/utils v0.0.0-20241104100929-3ea5e8cea738 // indirect
)
