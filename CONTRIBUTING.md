# Contributing

Anyone is welcome to contribute.

1. open an issue or discussion post to track the effort
2. fork this repository, then clone it
3. place this in your own module's `go.mod` to enable testing local changes
    - `replace github.com/k-capehart/go-salesforce/v2 => /path_to_local_fork/`
4. run format checks locally
    - `make install-tools`
    - `make fmt`
5. run tests
    - `make test`
    - `make test-ouput` (with html output)
    - note that [codecov](https://app.codecov.io/gh/k-capehart/go-salesforce) does not count partial lines so calculations may differ
6. linting
    - install [golangci-lint](https://golangci-lint.run/welcome/install/)
    - `make lint`
7. Create a PR and link the issue